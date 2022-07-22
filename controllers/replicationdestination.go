package controllers

import (
	"context"
	"fmt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotRestoreReconciler) CreateReplicationDestination(log logr.Logger) (bool, error) {

	// get volumesnapshotrestore from cluster
	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return false, err
	}

	// define replicationDestination to be created
	repDestination := &volsyncv1alpha1.ReplicationDestination{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-dest", vsr.Name),
			Namespace: r.NamespacedName.Namespace,
			Labels: map[string]string{
				VSRLabel: vsr.Name,
			},
		},
	}

	// Create ReplicationDestination in protected namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repDestination, func() error {

		return r.buildReplicationDestination(repDestination, &vsr)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repDestination,
			corev1.EventTypeNormal,
			"ReplicationDestinationReconciled",
			fmt.Sprintf("%s replicationdestination %s", op, repDestination.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) buildReplicationDestination(replicationDestination *volsyncv1alpha1.ReplicationDestination, vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore) error {

	// get restic secret created by controller
	dmresticSecretName := vsr.Spec.ResticSecretRef.Name
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: dmresticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	stringCapacity := vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Size
	capacity := resource.MustParse(stringCapacity)

	// build ReplicationDestination
	replicationDestinationSpec := volsyncv1alpha1.ReplicationDestinationSpec{
		Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{
			Manual: fmt.Sprintf("%s-trigger", vsr.Name),
		},
		Restic: &volsyncv1alpha1.ReplicationDestinationResticSpec{
			// TODO: create restic secret from secret from VSB CR status
			Repository: resticSecret.Name,
			ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				CopyMethod:  volsyncv1alpha1.CopyMethodSnapshot,
				// let replicationDestination create PVC
				Capacity: &capacity,
			},
		},
	}
	if replicationDestination.CreationTimestamp.IsZero() {
		replicationDestination.Spec = replicationDestinationSpec
	}

	return nil
}

func (r *VolumeSnapshotRestoreReconciler) WaitForReplicationDestinationToBeReady(log logr.Logger) (bool, error) {

	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return false, err
	}

	repDestName := fmt.Sprintf("%s-rep-dest", vsr.Name)
	repDest := volsyncv1alpha1.ReplicationDestination{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsr.Spec.ProtectedNamespace, Name: repDestName}, &repDest); err != nil {
		r.Log.Info("error getting replicationDestination")
		return false, err
	}

	if repDest.Status != nil {
		// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
		if len(repDest.Status.LastManualSync) > 0 && len(repDest.Spec.Trigger.Manual) > 0 {
			sourceStatus := repDest.Status.LastManualSync
			sourceSpec := repDest.Spec.Trigger.Manual
			if sourceStatus == sourceSpec {

				vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestoreVolSyncPhaseCompleted

				// Update VSR status as completed
				err := r.Status().Update(context.Background(), &vsr)
				if err != nil {
					return false, err
				}

				r.Log.Info("replicationDestination has completed")
				return true, nil
			}
		}
	}

	r.Log.Info("waiting for replicationDestination to complete")
	return false, nil
}
