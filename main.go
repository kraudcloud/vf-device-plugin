// When compiled for an Alpine container use
// CGO_ENABLED=0 go build

package main

import (
	"flag"
	"github.com/fsnotify/fsnotify"
	api "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"log"
	"strings"
	"syscall"
)

type vfInstance struct {
	devicePlugin *vfioDevicePlugin
	resourceName string
	iommuGroup   string
	pcieName     string
	socketName   string
}

func main() {
	var instances []vfInstance
	var configFile string

	flag.StringVar(&configFile, "config", "/root/config/config.yml", "path to the configuration file")
	flag.Parse()

	log.Print("Starting VFIO device plugin for Kubernetes")

	config := readConfigFile(configFile)
	_ = config

	devices := scanDevices()

	for _, dev := range devices {
		var instance vfInstance
		instance.devicePlugin = nil
		instance.iommuGroup = dev.iommuGroup
		instance.pcieName = dev.pciName
		instance.resourceName = "kr-vf/" + dev.pfEthName + "-vf" + dev.vfNumber
		instance.socketName = api.DevicePluginPath + strings.ReplaceAll(instance.resourceName, "/", "-") + ".sock"
		instances = append(instances, instance)
	}

	log.Print("Starting new FS watcher")
	watcher, err := newFSWatcher(api.DevicePluginPath)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	log.Print("Starting new OS watcher")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	restart := true

L:
	for {
		if restart {
			var err error

			for _, instance := range instances {
				if instance.devicePlugin != nil {
					instance.devicePlugin.Stop()
				}
			}

			for _, instance := range instances {
				instance.devicePlugin = NewDevicePlugin(&instance)
				err = instance.devicePlugin.Serve()
				if err != nil {
					log.Print("Failed to contact Kubelet, retrying")
					break
				}
			}

			if err != nil {
				continue
			}

			restart = false
		}

		select {
		case event := <-watcher.Events:
			if (event.Name == api.KubeletSocket) && (event.Op&fsnotify.Create) == fsnotify.Create {
				log.Printf("inotify: %s created, restarting", api.KubeletSocket)
				restart = true
			}
		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Print("Received SIGHUP, restarting.")
				restart = true
			default:
				log.Printf("Received signal '%v', shutting down", s)
				for _, instance := range instances {
					if instance.devicePlugin != nil {
						instance.devicePlugin.Stop()
					}
				}
				break L
			}
		}
	}
}
