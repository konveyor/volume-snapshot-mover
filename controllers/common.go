package controllers

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VSBLabel                      = "datamover.oadp.openshift.io/vsb"
	VSRLabel                      = "datamover.oadp.openshift.io/vsr"
	DummyPodImage                 = "quay.io/konveyor/rsync-transfer:latest"
	volumeSnapshotClassDefaultKey = "snapshot.storage.kubernetes.io/is-default-class"
	storageClassDefaultKey        = "storageclass.kubernetes.io/is-default-class"
)

// Restic secret data keys
const (
	// AWS vars
	AWSAccessKey     = "AWS_ACCESS_KEY_ID"
	AWSSecretKey     = "AWS_SECRET_ACCESS_KEY"
	AWSDefaultRegion = "AWS_DEFAULT_REGION"

	// TODO: GCP and Azure

	// Restic repo vars
	ResticPassword   = "RESTIC_PASSWORD"
	ResticRepository = "RESTIC_REPOSITORY"

	// Datamover annotation keys
	DatamoverResticRepository = "datamover.io/restic-repository"
	DatamoverSourcePVCName    = "datamover.io/source-pvc-name"
	DatamoverSourcePVCSize    = "datamover.io/source-pvc-size"
)

// Restic secret vars to create new secrets
var (
	AWSAccessValue        []byte
	AWSSecretValue        []byte
	AWSDefaultRegionValue []byte

	// TODO: GCP and Azure

	ResticPasswordValue []byte
	ResticRepoValue     string
)

type ReconcileFunc func(logr.Logger) (bool, error)

// reconcileBatch steps through a list of reconcile functions until one returns
// false or an error.
func ReconcileBatch(l logr.Logger, reconcileFuncs ...ReconcileFunc) (bool, error) {
	for _, f := range reconcileFuncs {
		if cont, err := f(l); !cont || err != nil {
			return cont, err
		}
	}
	return true, nil
}

func PopulateResticSecret(name string, namespace string, label string) (*corev1.Secret, error) {

	// define Restic secret to be created
	newResticSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", name),
			Namespace: namespace,
			Labels: map[string]string{
				label: name,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}

	return newResticSecret, nil
}

func BuildResticSecret(givensecret *corev1.Secret, secret *corev1.Secret, resticrepo string) error {

	// assign new restic secret values
	for key, val := range givensecret.Data {
		switch {
		case key == AWSAccessKey:
			AWSAccessValue = val
		case key == AWSSecretKey:
			AWSSecretValue = val
		case key == AWSDefaultRegion:
			AWSDefaultRegionValue = val
		case key == ResticPassword:
			ResticPasswordValue = val
		}
	}

	// build new Restic secret
	resticSecretData := &corev1.Secret{
		Data: map[string][]byte{
			AWSAccessKey:     AWSAccessValue,
			AWSSecretKey:     AWSSecretValue,
			AWSDefaultRegion: AWSDefaultRegionValue,
			ResticPassword:   ResticPasswordValue,
			ResticRepository: []byte(resticrepo),
		},
	}

	secret.Data = resticSecretData.Data
	return nil
}

// TODO: GCP & Azure validations
func ValidateResticSecret(resticsecret *corev1.Secret) error {
	if resticsecret == nil {
		return errors.New("empty restic secret. Please create a restic secret")
	}

	if resticsecret.Data == nil {
		return errors.New("secret data is empty")
	}
	for key, val := range resticsecret.Data {
		switch key {
		case AWSAccessKey:
			b := checkByteArrayIsEmpty(val)
			if !b {
				return errors.New("awsAccessKey value cannot be empty")
			}
		case AWSSecretKey:
			b := checkByteArrayIsEmpty(val)
			if !b {
				return errors.New("awsSecretKey value cannot be empty")
			}
		case ResticPassword:
			b := checkByteArrayIsEmpty(val)
			if !b {
				return errors.New("resticPassword value cannot be empty")
			}
		case ResticRepository:
			b := checkByteArrayIsEmpty(val)
			if !b {
				return errors.New("resticRepository value cannot be empty")
			}
		}
	}
	return nil
}

func checkByteArrayIsEmpty(val []byte) bool {

	return len(val) != 0
}
