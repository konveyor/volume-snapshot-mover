package controllers

import (
	"context"
	"errors"
	"fmt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Restic secret data keys
const (

	// VolumeSnapshotMover annotation keys
	SnapMoverResticRepository = "datamover.io/restic-repository"
	SnapMoverSourcePVCName    = "datamover.io/source-pvc-name"
	SnapMoverSourcePVCSize    = "datamover.io/source-pvc-size"
)

func (r *VolumeSnapshotBackupReconciler) CreateVSBResticSecret(log logr.Logger) (bool, error) {
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
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Name: pvcName, Namespace: vsb.Spec.ProtectedNamespace}, &pvc); err != nil {
		r.Log.Error(err, "unable to fetch PVC")
		return false, err
	}

	// get restic secret name
	credName := vsb.Spec.ResticSecretRef.Name
	if credName == "" {
		return false, errors.New("restic secret name cannot be empty")
	}

	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: credName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return false, err
	}

	err := ValidateResticSecret(&resticSecret)
	if err != nil {
		r.Log.Error(err, "Restic Secret is malformed")
		return false, err
	}

	for key, val := range resticSecret.Data {
		if key == ResticRepository {
			// if trailing '/' in user-created Restic repo, remove it
			stringVal := string(val)
			stringVal = strings.TrimRight(stringVal, "/")

			ResticRepoValue = stringVal
		}
	}
	resticrepo := fmt.Sprintf("%s/%s/%s", ResticRepoValue, pvc.Namespace, pvc.Name)

	rsecret, err := PopulateResticSecret(vsb.Name, vsb.Spec.ProtectedNamespace, VSBLabel)
	if err != nil {
		return false, err
	}

	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, rsecret, func() error {

		return BuildResticSecret(&resticSecret, rsecret, resticrepo)
	})
	if err != nil {
		return false, err
	}

	// set created Restic repo to VSB status
	vsb.Status.ResticRepository = string(rsecret.Data[ResticRepository])

	// Update VSB status
	err = r.Status().Update(context.Background(), &vsb)
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(rsecret,
			corev1.EventTypeNormal,
			"ResticSecretBackupReconciled",
			fmt.Sprintf("%s restic secret %s", op, rsecret.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) CreateVSRResticSecret(log logr.Logger) (bool, error) {
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

	// get restic secret name
	// get restic secret name
	credName := vsr.Spec.ResticSecretRef.Name
	if credName == "" {
		return false, errors.New("restic secret name cannot be empty")
	}
	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: credName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return false, err
	}

	err := ValidateResticSecret(&resticSecret)
	if err != nil {
		r.Log.Error(err, "Restic Secret is malformed")
		return false, err
	}
	// define Restic secret to be created
	newResticSecret, err := PopulateResticSecret(vsr.Name, vsr.Spec.ProtectedNamespace, VSRLabel)
	if err != nil {
		return false, err
	}
	resticrepo := vsr.Spec.VolumeSnapshotMoverBackupref.ResticRepository
	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, newResticSecret, func() error {

		return BuildResticSecret(&resticSecret, newResticSecret, resticrepo)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(newResticSecret,
			corev1.EventTypeNormal,
			"ResticSecretRestoreReconciled",
			fmt.Sprintf("%s restic secret %s", op, newResticSecret.Name),
		)
	}
	return true, nil
}
