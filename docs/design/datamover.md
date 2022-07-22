---
title: datamover_crd_design
authors:
  - "@savitharaghunathan"
reviewers:
  - "@dymurray"
  - "@eemcmullan"
  - "@shubham-pampattiwar"
approvers:
  - "@dymurray"
  - "@eemcmullan"
  - "@shubham-pampattiwar"
creation-date: 2022-05-03
status: provisional
---

# Data Mover CRD design

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created


## Summary
Velero currently supports backup and restore of applications backed by CSI volumes by leveraging the CSI plugin. The problem with CSI snapshots on some providers is that these snapshots are local to the K8s cluster and cannot be recovered if the cluster gets deleted accidentally or if there is a disaster. In order to overcome this issue, a Data Mover solution is made available for users to save the snapshots in a remote storage. 

## Motivation

Create an extensible design to support various data movers that can be integrated with the extended Velero CSI plugin. Vendors should be able to bring their own data mover controller and implementation, and use that with Velero CSI plugin.

## Goals
* Create an extensible data mover solution
* Supply a default data mover option 
* Supply APIs for DataMover CRs (eg: VolumeSnapshotBackup, VolumeSnapshotRestore, DataMoverClass)
* Supply a sample codebase for the Data Mover plugin and controller implementation


## Non Goals
* Maintain 3rd party data mover implementations
* Adding a status watch controller to Velero

## User stories

Story 1: 
As an application developer, I would like to save the CSI snaphots in a S3 bucket. 

Story 2:
As a cluster admin, I would like to be able to restore CSI snapshots if disaster happens.

## Design & Implementation details

This design supports adding the data mover feature to the Velero CSI plugin and facilitates integrating various vendor implemented data movers. 

![DataMover CRD](../images/datamovercrd.png)

Note: We will be supporting VolSync as the default data mover. 

There will be two controllers - VolumeSnapshotBackup & VolumeSnapshotRestore. The VolumeSnapshotBackup Controller will be responsible for reconciling VolumeSnapshotBackup CR. Likewise, VolumeSnapshotRestore Controller will watch for VolumeSnapshotRestore CR. Both of these CRs will have a reference to a DataMoverClass. 

`DataMoverClass` is a cluster scoped Custom Resource that will have details about the data mover. The specified mover will be registered in the system by creating the datamoverclass CR, addig a velero plugin that will create the appropriate resources for datamovement of a single datamoverclass and a controller that will reconcile the objects created by the plugin. The datamoverclass spec will also include a field (`selector`) to identify the PVCs that would be moved with the given data mover.


```
apiVersion: volumesnapshotmover.datamover.io/v1alpha1
kind: DataMoverClass
metadata:
  annotations:
    volumesnapshotmover.datamover.io/default: "true"
  name: <name>
spec:
  mover: <VolSync>
  selector: <tagname>

```

The above `DataMoverClass` name will be referenced in `VolumeSnapshotBackup` & `VolumeSnapshotRestore` CRs. This will help in selecting the data mover implementation during runtime. If the `DataMoverClass` name is not defined, then the default `DataMoverClass` will be used, which in this case will be `VolSync`

### Data Mover Backup

When a velero backup is created, it triggers the custom velero CSI plugin plugin (velero BackupItemAction plugin) to create the `VolumeSnapshotBackup` CR in the app namespace. The extended plugin looks up for the PVCs in the user namespace mentioned in the velero backup and creates a `VolumeSnapshotBackup` CR for every PVC in that namespace that is filtered by the `datamoverclass.spec.selector`.


```
apiVersion: volumesnapshotmover.datamover.io/v1alpha1
kind: VolumeSnapshotBackup
metadata:
  name: <name>
spec:
  protectedNamespace: <ns>
  dataMoverClass: <DataMoverClass name> 
  dataSourceRef:
    apiGroup: <APIGroup>
    kind: <PVC|VolumeSnapshotContent>
    name: <name>
  config:  //optional based on the datamover impl

```
### Data Mover Restore
When a velero restore is triggered, the custom Velero CSI plugin looks for `VolumeSnapshotBackup` in the backup resources. If it encounters a `VolumeSnapshotBackup` resource, then the extended plugin (velero RestoreItemAction plugin) will create a `VolumeSnapshotRestore` CR in the app namespace. It will populate the CR with the details obtained from the `VolumeSnapshotBackup` resource. 

```
apiVersion: volumesnapshotmover.datamover.io/v1alpha1
kind: VolumeSnapshotRestore
metadata:
  name: <name>
spec:
  protectedNamespace: <ns>
  resticSecretRef:  // optional
    name: dm-restic-secret  
  dataMoverBackupRef:
    sourcePVCData: 
      name: <pvc_name>
      size: <size>
    resticrepository: <restic_repo>
```
Config section in the above CR is optional. It lets the user specify extra parameters needed by the data mover. For eg: VolSync data mover needs restic secret to perform backup & restore

eg: 

```
apiVersion: volumesnapshotmover.datamover.io/v1alpha1
kind: VolumeSnapshotRestore
metadata:
  name: <name>
spec:
  protectedNamespace: <ns>
  resticSecretRef:  
    name: dm-restic-secret  
  dataMoverBackupRef:
    sourcePVCData: 
      name: <pvc_name>
      size: <size>
    resticrepository: <restic_repo>
```

We will provide a sample codebase which the vendors will be able to extend and implement their own data movers. 


### Default Data Mover controller

VolSync will be used as the default Data Mover for this PoC and `restic` will be the supported method for backup & restore of PVCs. Restic repository details are configured in a `secret` object which gets used by the VolSync's resources. This design takes advantage of VolSync's two resources - `ReplicationSource` & `ReplicationDestination`. `ReplicationSource` object helps with taking a backup of the PVCs and using restic to move it to the storage specified in the restic secret. `ReplicationDestination` object takes care of restoring the backup from the restic repository. There will be a 1:1 relationship between the replication src/dest CRs and PVCs.

We will follow a two phased approach for implementation of this controller. For phase 1, the user will create a restic secret. Using that secret as source, the controller will create on-demand secrets for every backup/restore request. For phase 2, the user will provide the restic repo details. This may be an encryption password and BSL reference, and the controller will create restic secret using BSL info, or they can supply their own backup target repo and access credentials. We will be focussing on phase 1 approach for this design.

The user creates a restic secret with all the following details,

```
apiVersion: v1
kind: Secret
metadata:
  name: restic-config
type: Opaque
stringData:
  # The repository url
  RESTIC_REPOSITORY: s3:s3.amazonaws.com/<bucket>
  # The repository encryption key
  RESTIC_PASSWORD: <password>
  # ENV vars specific to the chosen back end
  # https://restic.readthedocs.io/en/stable/030_preparing_a_new_repo.html
  AWS_ACCESS_KEY_ID: <access_id>
  AWS_SECRET_ACCESS_KEY: <access_key>
```
*Note: More details for installing restic secret in [here](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#specifying-a-repository)*


Custom velero CSI plugin will be responsible for creating `VolumeSnapshotBackup` & `VolumeSnapshotRestore` CRs. 

Once a VolumeSnapshotBackup CR gets created, the controller will create the corresponding `ReplicationSource` CR in the protected namespace. VolSync watches for the creation of `ReplicationSource` CR and copies the PVC data to the restic repository mentioned in the `dm-restic-secret`.  
```
apiVersion: volsync.backube/v1alpha1
kind: ReplicationSource
metadata:
  name: database-source
  namespace: openshift-adp
spec:
  sourcePVC: <pvc_name>
  trigger:
    manual: <trigger_name>
  restic:
    pruneIntervalDays: 15
    repository: restic-config
    retain:
      hourly: 1
      daily: 1
      weekly: 1
      monthly: 1
      yearly: 1
    copyMethod: None
```

Similarly, when a VolumeSnapshotRestore CR gets created, controller will create a `ReplicationDestination` CR in the protected namespace. VolSync controller copies the PVC data from the restic repository to the protected namespace and creates a volumesnapshot, which in turn gets referenced as the datasource for PVC.

```
apiVersion: volsync.backube/v1alpha1
kind: ReplicationDestination
metadata:
  name: <protected_namespace>
spec:
  trigger:
    manual: <trigger_name>
  restic:
    destinationPVC: <pvc_name>
    repository: restic-config
    copyMethod: None
```

A status controller is created to watch VolSync CRs. It watches the `ReplicationSource` and`ReplicationDestination` objects and updates VolumeSnapShot CR events. 

*Note: Potential feature addition to Velero: A status watch controller for DataMover CRs. This can be used to update Velero Backup/Restore events with the DataMover CR results*

Data mover controller will clean up all controller-created resources after the process is complete.


### Support for multiple data mover plugins
`DataMoverClass` spec will support the following field,
    `selector: <tagname>`
PVC must be labelled with the `<tagname>`, to be moved by the specific `DataMoverClass`. User/Admin of the cluster must label the PVCs with the required `<tagname>` and map it to a `DataMoverClass`. If the PVCs are not labelled, it will be moved by the default datamover.

#### Alternate options
PVCs can be annotated with the `DataMoverClass`, and when a backup is created, the controller will look at the DataMoverClass and add it to the `VolumeSnapshotBackup` CR. 


