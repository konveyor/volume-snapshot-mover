# Develop and Test VolumeSnapshotMover Controller

## VolumeSnapshotMover Backup

- To use the volumesnapshotmover backup controller, you will first need a volumeSnapshot. This can be achieved
by using the Velero CSI plugin during backup of the stateful application.

- Install OADP Operator 

- Have a stateful application running in a separate namespace. 

- [Install](https://volsync.readthedocs.io/en/stable/installation/index.html) the VolSync controller.

```
$ helm repo add backube https://backube.github.io/helm-charts/
$ helm install -n openshift-adp volsync backube/volsync
```

- Configure a [restic secret](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#id2)

```
apiVersion: v1
kind: Secret
metadata:
  name: dm-restic-secret
type: Opaque
stringData:
  # The repository url (if using AWS s3)
  RESTIC_REPOSITORY: s3:s3.amazonaws.com/<bucket>/data-mover-snapshots
  # The repository encryption key
  RESTIC_PASSWORD: my-secure-restic-password
  AWS_ACCESS_KEY_ID: <bucket_access_key_id>
  AWS_SECRET_ACCESS_KEY: <bucket_secret_access_key>
```

`oc create -n <protected-namespace> -f ./restic-secret.yaml`

- Install the VolumeSnapshotMover CRDs `VolumeSnapshotBackup` and `VolumeSnapshotRestore` using: `oc create -f config/crd/bases/`

- Run the controller by executing `make run`

- Create a `VolumeSnapshotBackup` CR that looks similar to below in the application namespace.
  - The VolumeSnapshotContent `name` is the name of the content created
  during backup using the Velero CSI plugin.

```
apiVersion: datamover.oadp.openshift.io/v1alpha1
kind: VolumeSnapshotBackup
metadata:
  name: <vsb-name>
spec:
  volumeSnapshotContent:
    name: <snapcontent-name>
  protectedNamespace: <adp-namespace>

```

`oc create -n <application-namespace> -f vsb.yaml`

- When volumesnapshotmover backup is completed, you should have a snapshot in your Restic repository.

## VolumeSnapshotMover Restore

- Have a completed backup, as well as a snapshot in a Restic repository by following the above steps.

- If needed, create a Restic secret named `dm-restic-secret` in the protected namespace.

- Run the controller by executing `make run`

- Create a `VolumeSnapshotRestore` CR that looks similar to below:

```
apiVersion: datamover.oadp.openshift.io/v1alpha1
kind: VolumeSnapshotRestore
metadata:
  name: <vsr-name>
spec:
  protectedNamespace: <protected-ns>
  resticSecretRef: 
    name: dm-restic-secret
  volumeSnapshotMoverBackupRef:
    sourcePVCData: 
      name: <source-pvc-name>
      size: <source-pvc-size>
    resticrepository: <your-restic-repo>
```

`oc create -n <application-namespace> -f vsr.yaml`

- When volumesnapshotmover restore is completed, there should be a VolSync `ReplicationDestination`,
  as well as a snapshot, in the protected namespace.
