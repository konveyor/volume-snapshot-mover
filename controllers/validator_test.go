package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestVolumeSnapshotMoverBackupReconciler_ValidateVolumeSnapshotMoverBackup(t *testing.T) {
	tests := []struct {
		name    string
		vsb     *volsnapmoverv1alpha1.VolumeSnapshotBackup
		vsc     *snapv1.VolumeSnapshotContent
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Given valid VSB CR -> no validation errors",
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
				},
			},
			vsc: &snapv1.VolumeSnapshotContent{
				ObjectMeta: v1.ObjectMeta{
					Name: "sample-snapshot",
				},
				Spec: snapv1.VolumeSnapshotContentSpec{
					VolumeSnapshotRef: corev1.ObjectReference{
						Name: "sample-vs",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Given an invalid VSB CR ->  validation errors",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{},
					ProtectedNamespace:    "foo",
				},
			},
			vsc: &snapv1.VolumeSnapshotContent{
				ObjectMeta: v1.ObjectMeta{
					Name: "sample-snapshot",
				},
				Spec: snapv1.VolumeSnapshotContentSpec{
					VolumeSnapshotRef: corev1.ObjectReference{
						Name: "sample-vs",
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given an invalid VSC ->  validation errors",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot-vsc",
					},
					ProtectedNamespace: "foo",
				},
			},
			vsc: &snapv1.VolumeSnapshotContent{
				ObjectMeta: v1.ObjectMeta{
					Name: "sample-snapshot",
				},
				Spec: snapv1.VolumeSnapshotContentSpec{
					VolumeSnapshotRef: corev1.ObjectReference{
						Name: "sample-vs",
					},
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.vsb, tt.vsc)
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
			got, err := r.ValidateVolumeSnapshotMoverBackup(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotMoverBackupReconciler.ValidateVolumeSnapshotMoverBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VolumeSnapshotMoverBackupReconciler.ValidateVolumeSnapshotMoverBackup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVolumeSnapshotMoverRestoreReconciler_ValidateVolumeSnapshotMoverRestore(t *testing.T) {
	tests := []struct {
		name    string
		vsr     *volsnapmoverv1alpha1.VolumeSnapshotRestore
		wantErr bool
		want    bool
	}{
		// TODO: Add test cases.
		{
			name: "valid VSR -> no validation errors",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "resticSecret",
					},
					ProtectedNamespace: "foo",
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "empty protected ns -> no validation errors",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "resticSecret",
					},
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty restic repository -> validation errors",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "resticSecret",
					},
					ProtectedNamespace: "foo",
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						ResticRepository: "",
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty pvc name -> validation errors",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "resticSecret",
					},
					ProtectedNamespace: "foo",
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty pvc size -> validation errors",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "resticSecret",
					},
					ProtectedNamespace: "foo",
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty secret ->  validation errors",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef:    corev1.LocalObjectReference{},
					ProtectedNamespace: "foo",
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.vsr)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &VolumeSnapshotRestoreReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),

				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.vsr.Namespace,
						Name:      tt.vsr.Name,
					},
				},
			}
			got, err := r.ValidateVolumeSnapshotMoverRestore(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotMoverRestoreReconciler.ValidateVolumeSnapshotMoverRestore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VolumeSnapshotMoverRestoreReconciler.ValidateVolumeSnapshotMoverRestore() = %v, want %v", got, tt.want)
			}
		})
	}
}
