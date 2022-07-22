package controllers

import (
	"context"
	"errors"
	"fmt"
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotBackupReconciler) CreateReplicationSource(log logr.Logger) (bool, error) {
	// get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	clonedPVC := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: pvcName}, &clonedPVC); err != nil {
		r.Log.Error(err, "unable to fetch cloned PVC")
		return false, err
	}

	// define replicationSource to be created
	repSource := &volsyncv1alpha1.ReplicationSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-src", vsb.Name),
			Namespace: vsb.Spec.ProtectedNamespace,
			Labels: map[string]string{
				VSBLabel: vsb.Name,
			},
		},
	}

	// Create ReplicationSource in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repSource, func() error {

		return r.buildReplicationSource(repSource, &vsb, &clonedPVC)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repSource,
			corev1.EventTypeNormal,
			"ReplicationSourceReconciled",
			fmt.Sprintf("%s replicationsource %s", op, repSource.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) buildReplicationSource(replicationSource *volsyncv1alpha1.ReplicationSource, vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, pvc *corev1.PersistentVolumeClaim) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", vsb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	// build ReplicationSource
	replicationSourceSpec := volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: pvc.Name,
		Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			Manual: fmt.Sprintf("%s-trigger", vsb.Name),
		},
		Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
			Repository: resticSecret.Name,
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod:              volsyncv1alpha1.CopyMethodNone,
				StorageClassName:        &vsb.Status.SourcePVCData.StorageClassName,
				VolumeSnapshotClassName: &vsb.Status.VolumeSnapshotClassName,
			},
		},
	}
	if replicationSource.CreationTimestamp.IsZero() {
		replicationSource.Spec = replicationSourceSpec
	}

	return nil
}

func (r *VolumeSnapshotBackupReconciler) setVSBRepSourceStatus(log logr.Logger) (bool, error) {

	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return false, err
	}

	repSourceName := fmt.Sprintf("%s-rep-src", vsb.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: repSourceName}, &repSource); err != nil {
		if k8serror.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if repSource.Status == nil {
		r.Log.Info("replication source is yet to have a status")
		return false, nil
	}

	if repSource.Status != nil {

		// check for ReplicationSource phase
		repSourceCompleted, err := r.isRepSourceCompleted(&vsb)
		if err != nil {
			return false, err
		}
		reconCondition := metav1.Condition{}
		reconConditionProgress := metav1.Condition{}
		for i := range repSource.Status.Conditions {
			if repSource.Status.Conditions[i].Type == "Reconciled" {
				reconCondition = repSource.Status.Conditions[i]
			}
			if repSource.Status.Conditions[i].Reason == volsyncv1alpha1.SynchronizingReasonSync {
				reconConditionProgress = repSource.Status.Conditions[i]
			}
		}

		if repSourceCompleted && reconCondition.Status == metav1.ConditionTrue {

			// Update VSB status as completed
			vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverVolSyncPhaseCompleted
			err := r.Status().Update(context.Background(), &vsb)
			if err != nil {
				return false, err
			}
			r.Log.Info("marking volumesnapshotbackup VolSync phase as complete")
			return true, nil

			// ReplicationSource phase is still in progress
		} else if !repSourceCompleted && reconConditionProgress.Type == volsyncv1alpha1.ConditionSynchronizing {
			vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhaseInProgress

			// Update VSB status as in progress
			err := r.Status().Update(context.Background(), &vsb)
			if err != nil {
				return false, err
			}
			r.Log.Info("marking volumesnapshotbackup as in progress, vsb recon as false")
			return false, nil

			//if not in progress or completed, phase failed
		} else {
			vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhaseFailed

			// Update VSB status
			err := r.Status().Update(context.Background(), &vsb)
			if err != nil {
				return false, err
			}
			r.Log.Info("marking volumesnapshotbackup as failed, vsb recon as false")
			return false, nil
		}
	}
	return false, errors.New("replication source status not ready")
}

func (r *VolumeSnapshotBackupReconciler) isRepSourceCompleted(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup) (bool, error) {

	// get replicationsource
	repSourceName := fmt.Sprintf("%s-rep-src", vsb.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: repSourceName}, &repSource); err != nil {
		r.Log.Info("error here isRepSourceCompleted")
		return false, err
	}

	if repSource.Status != nil {
		// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
		if len(repSource.Status.LastManualSync) > 0 && len(repSource.Spec.Trigger.Manual) > 0 {
			sourceStatus := repSource.Status.LastManualSync
			sourceSpec := repSource.Spec.Trigger.Manual
			if sourceStatus == sourceSpec {
				return true, nil
			}
		}
	}

	// ReplicationSource has not yet completed but is not failed
	return false, nil
}
