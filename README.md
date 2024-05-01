# Kubernetes VF Device Plugin


allows passing a network devices VFs as exact names into kubevirt without a CNI.
CNIs do things we dont need and confuse cilium and kubevirts builtin device plugin picks a random vf.
instead this plugin registers each VF as a separate resource to k8s

it is assumed that all VFs are bound to vfio-pcie and preconfigured with mac filters on the host.
the plugin doesnt do any of that. it only creates the glue to make them available to kubevirt

after installing the plugin you get in node resources:

```yaml
Allocatable:
  kr-vf/eth0-00.1:                 1
  kr-vf/eth0-00.2:                 1
  kr-vf/eth0-00.3:                 1
  kr-vf/eth0-00.4:                 1
  kr-vf/eth0-00.5:                 1
  kr-vf/eth0-00.6:                 1
  kr-vf/eth0-00.7:                 1
  kr-vf/eth0-01.0:                 1
```


which can be passed into kubevirt as
```yaml
domain:
  devices:
    hostDevices:
    - deviceName: kr-vf/eth0-00.3
      name: eth0
```


uses code borrowed from https://github.com/mrlhansen/vfio-device-plugin

