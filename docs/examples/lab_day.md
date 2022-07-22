# OADP Data Mover Lab

Data mover make use of a custom Velero [CSI plugin](https://github.com/openshift/velero-plugin-for-csi/tree/data-mover) 
and [VolSync](https://volsync.readthedocs.io/en/stable/) to take a snapshot
of a stateful application and relocate this snapshot into an object store.   
The snapshot can then be used during a restore process to recover stateful 
application data in instances such as cluster deletion or disaster. 

## Steps for Data Mover with Rocket Chat

* For this example, you will deploy [rocket chat](https://github.com/konveyor/mig-demo-apps/tree/master/apps/rocket-chat).
* Add data to the rocket chat application.
* Using the custom CSI plugin, take a backup of the application.
* Uh oh, your application namespace and snapshot have been deleted!
* Using the custom CSI plugin, create a restore of the application. Disaster adverted.

## Prerequisites
* Install OADP Operator using [these steps](https://github.com/openshift/oadp-operator/blob/master/docs/developer/install_from_source.md).

* [Install](https://volsync.readthedocs.io/en/stable/installation/index.html) the VolSync controller:

    `$ helm repo add backube https://backube.github.io/helm-charts/`  
    `$ helm install -n openshift-adp volsync backube/volsync`

* Create a StorageClass and VolumeShapshotClass:

- A `StorageClass` and a `VolumeSnapshotClass` are needed before the Rocket Chat application
is created. The app will map to the `StorageClass`, which contains information about the CSI driver.

- Include a label in `VolumeSnapshotClass` to let
Velero know which to use, and set `deletionPolicy` to  `Retain` in order for
`VolumeSnapshotContent` to remain after the application namespace is deleted.

- *Note:* Make sure you only have one default volumeSnapshotClass and StorageClass!

`oc create -f docs/examples/manifests/mysql/VolumeSnapshotClass.yaml`

```
apiVersion: v1
kind: List
items:
  - apiVersion: snapshot.storage.k8s.io/v1
    kind: VolumeSnapshotClass
    metadata:
      name: example-snapclass
      labels:
        velero.io/csi-volumesnapshot-class: 'true'
      annotations:
        snapshot.storage.kubernetes.io/is-default-class: 'true'
    driver: ebs.csi.aws.com
    deletionPolicy: Retain
```

`gp2-csi` comes as a default `StorageClass` with OpenShift clusters.

`oc get storageclass`

If this is not found, create a `StorageClass` like below:

```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp2-csi
  annotations:
    storageclass.kubernetes.io/is-default-class: 'true'
provisioner: ebs.csi.aws.com
parameters:
  type: gp2
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

* Have a DPA CR such as below. Note the `enableDataMover` boolean field. It is specified under `spec.features` . This CR will deploy our volume-snapshot-mover
controller as well as the modified CSI plugin. Make sure you replace the object storage details appropriately.


```
apiVersion: oadp.openshift.io/v1alpha1
kind: DataProtectionApplication
metadata:
  name: velero-sample
  namespace: openshift-adp
spec:
  features:
    enableDataMover: true
  backupLocations:
    - velero:
        config:
          profile: default
          region: us-east-1
        credential:
          key: cloud
          name: cloud-credentials
        default: true
        objectStorage:
          bucket: bucket-name
          prefix: bucket-prefix
        provider: aws
  configuration:
    restic:
      enable: false
    velero:
      defaultPlugins:
        - openshift
        - aws
        - csi
      featureFlags:
        - EnableCSI
  unsupportedOverrides:
    veleroImageFqin: 'quay.io/emcmulla/velero:test2'
```

* Create a [restic secret](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#id2) named `dm-restic-secret` in the adp namespace:

  `$ oc create -n openshift-adp -f ./restic-secret.yaml`

## Deploy Rocket Chat

* Follow the steps in the Rocket Chat [README](https://github.com/konveyor/mig-demo-apps/blob/master/apps/rocket-chat/README.md).

## Add data to Rocket Chat

* You can add data to Rocket Chat by first finding the application route:

    `$ oc get route rocket-chat -n rocket-chat -ojsonpath="{.spec.host}"`

* Once at the application UI, create an account with your information. 

![Rocket_chat_home](/docs/examples/images/rocket_chat_home.png)

* Next, add a message in one of the channels.

![Rocket_chat_backup](/docs/examples/images/message.png)
        

## Create a backup using the custom CSI plugin

* The Velero CSI plugin has been extended to support data mover backup.  
To use these additional features, you must add the `enableDataMover` flag 
to your OADP DPA, as shown above. 

* Create a backup using the custom Velero CSI plugin:

```
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: sample-backup
spec:
  includedNamespaces:
  - rocket-chat
  storageLocation: velero-sample-1
```

`$ oc create -f backup.yaml -n openshift-adp`

* Once the backup is completed, we can make sure `volumeSnapshotBackup` status has completed:

`$ oc get volumesnapshotbackup <VSB_name> -n rocket-chat -o yaml`

```
apiVersion: datamover.oadp.openshift.io/v1alpha1
kind: VolumeSnapshotBackup
metadata:
  name: vsb-velero-rocketchat-data-claim-wf6x5
  namespace: rocket-chat
spec:
  protectedNamespace: openshift-adp
  volumeSnapshotContent:
    name: <velero_snapcontent_name>
status:
  phase: Completed
  resticrepository: <VSB_repo_name>
  sourcePVCData:
    name: rocketchat-data-claim
    size: 10Gi
```

* We can also check that we now have a snapshot in our remote storage. Example with aws s3:

`$ aws s3 ls <repo_name>/openshift-adp/<pvc_name>/snapshots/`

## Delete the Rocket Chat namespace and VolumeSnapshotContent

`$ oc delete ns rocket-chat`


* Get the VSC created by the Velero backup

`$ oc get volumesnapshotcontent`

* Delete this VSC. *Note:* you may have to edit `oc edit volumesnapshotcontent <vsc_name>` the VSC 
and remove the finalizer in order for this VSC to delete.

`$ oc delete volumesnapshotcontent <vsc_name>`

## Restore Rocket Chat using the custom CSI plugin

* The Velero CSI plugin has also been extended to support data mover restore.  
To use these additional features, your OADP DPA must have the `enableDataMover` flag.

* You must also add an `unsupportedOverride` for the Velero image 
to execute on this CR. This image can be found above in the example DPA.

```
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: sample-restore
  namespace: openshift-adp
spec:
  backupName: sample-backup
  restorePVs: true
```

`$ oc create -f restore.yaml -n openshift-adp`

## Check for successful restore

* Once the restore is completed, we can make sure `volumeSnapshotRestore` status has completed:

`$ oc get volumesnapshotrestore <VSR_name> -n rocket-chat -o yaml`

```
apiVersion: datamover.oadp.openshift.io/v1alpha1
kind: VolumeSnapshotRestore
metadata:
  name: vsr-rocketchat-data-claim
  namespace: rocket-chat
spec:
  dataMoverBackupRef:
    resticrepository: <VSR_repo_name>
    sourcePVCData:
      name: rocketchat-data-claim
      size: 10Gi
  protectedNamespace: openshift-adp
  resticSecretRef:
    name: dm-restic-secret
status:
  completed: true
  conditions:
  - lastTransitionTime: "2022-06-07T16:29:25Z"
    message: Reconcile complete
    reason: Complete
    status: "True"
    type: Reconciled
  snapshotHandle: snap-08d70e6a8ed9685dc
```

* Now navigate to the app's page and check that the data that was added before backup has been restored:

`$ oc get route rocket-chat -n rocket-chat -ojsonpath="{.spec.host}"`

![Rocket_chat_restore](/docs/examples/images/message.png)

