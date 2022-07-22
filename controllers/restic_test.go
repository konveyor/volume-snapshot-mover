package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestVolumeSnapshotBackupReconciler_CreateVSBResticSecret(t *testing.T) {
	tests := []struct {
		name             string
		vsb              *volsnapmoverv1alpha1.VolumeSnapshotBackup
		secret, rpsecret *corev1.Secret
		pvc              *corev1.PersistentVolumeClaim
		want             bool
		wantErr          bool
	}{
		// TODO: Add test cases.
		{
			name: "Given invalid pvc -> error in restic secret creation",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "bsl1-volysnc",
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot",
					Namespace: "foo",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "bsl1-volysnc",
					Namespace: namespace,
				},
			},
			secret:  &corev1.Secret{},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given valid vsb,restic secret -> successful creation of pvc specific restic secret",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "bsl2-name",
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot-pvc",
					Namespace: "foo",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "bsl2-name",
					Namespace: namespace,
				},
				Data: secretData,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Given invalid base secret -> error in restic secret creation",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "volsync-restic",
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot-pvc",
					Namespace: "foo",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "volsync-restic",
					Namespace: namespace,
				},
				Data: make(map[string][]byte),
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.vsb, tt.pvc, tt.rpsecret, tt.secret)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &VolumeSnapshotBackupReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),
				NamespacedName: types.NamespacedName{
					Namespace: tt.vsb.Spec.ProtectedNamespace,
					Name:      tt.vsb.Name,
				},
				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.vsb.Namespace,
						Name:      tt.vsb.Name,
					},
				},
			}
			got, err := r.CreateVSBResticSecret(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotBackupReconciler.CreateVSBResticSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VolumeSnapshotBackupReconciler.CreateVSBResticSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVolumeSnapshotRestoreReconciler_CreateVSRResticSecret(t *testing.T) {
	tests := []struct {
		name    string
		vsr     *volsnapmoverv1alpha1.VolumeSnapshotRestore
		secret  *corev1.Secret
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "valid case",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ProtectedNamespace: "foo",
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "bsl-volsync",
					},
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
						ResticRepository: "sample-repo",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "bsl-volsync",
					Namespace: "foo",
				},
				Data: secretData,
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.vsr, tt.secret)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &VolumeSnapshotRestoreReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),
				NamespacedName: types.NamespacedName{
					Namespace: tt.vsr.Spec.ProtectedNamespace,
					Name:      tt.vsr.Name,
				},
				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.vsr.Namespace,
						Name:      tt.vsr.Name,
					},
				},
			}
			got, err := r.CreateVSRResticSecret(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotRestoreReconciler.CreateVSRResticSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VolumeSnapshotRestoreReconciler.CreateVSRResticSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
