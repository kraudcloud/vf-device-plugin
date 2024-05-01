package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type vfioDevice struct {
	pciName    string
	deviceId   string
	vendorId   string
	iommuGroup string
	pfEthName  string
	vfNumber   string
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if strings.EqualFold(v, str) {
			return true
		}
	}

	return false
}

func scanDevices() []vfioDevice {
	var names []string
	var devices []vfioDevice

	path := "/sys/bus/pci/drivers/vfio-pci"
	bdf := regexp.MustCompile(`^[a-f0-9]{4}:[a-f0-9]{2}:[a-f0-9]{2}.[0-9]$`)
	iommu := regexp.MustCompile(`\/(\d+)$`)

	list, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range list {
		mode := file.Mode()
		name := file.Name()
		if (mode & os.ModeSymlink) == os.ModeSymlink {
			if bdf.MatchString(name) {
				names = append(names, name)
			}
		}
	}

	for _, name := range names {
		fullpath := path + "/" + name

		content, err := os.ReadFile(fullpath + "/vendor")
		if err != nil {
			log.Print(err)
			continue
		}
		vendor := strings.TrimSpace(string(content))
		vendor = vendor[2:]

		content, err = os.ReadFile(fullpath + "/device")
		if err != nil {
			log.Print(err)
			continue
		}
		device := strings.TrimSpace(string(content))
		device = device[2:]

		dest, err := os.Readlink(fullpath + "/iommu_group")
		if err != nil {
			log.Print(err)
			continue
		}

		match := iommu.FindStringSubmatch(dest)
		if len(match) == 0 {
			log.Print("Failed to get IOMMU group")
			continue
		}
		dest = match[1]

		if _, err := os.Stat("/dev/vfio/" + dest); os.IsNotExist(err) {
			log.Print(err)
			continue
		}

		// get pf ethernet device name , if exists
		var pfEthName string
		physfnNetDir := "/sys/bus/pci/drivers/vfio-pci/" + name + "/physfn/net/"
		entries, err := os.ReadDir(physfnNetDir)
		if err != nil {
			log.Print(err)
			continue
		}

		if len(entries) < 1 {
			log.Println("skipping " + name + " because no pf ethernet device")
			continue
		}

		pfEthName = filepath.Base(entries[0].Name())
		pfEthName = strings.TrimSuffix(pfEthName, ":")

		log.Print("Found PCI device " + name)
		log.Print("Vendor " + vendor)
		log.Print("Device " + device)
		log.Print("IOMMU Group " + dest)
		log.Print("PF ethernet device name " + pfEthName)

		split := strings.Split(name, ":")
		vfNumber := split[len(split)-1]

		devices = append(devices, vfioDevice{
			pciName:    name,
			vendorId:   vendor,
			deviceId:   device,
			iommuGroup: dest,
			pfEthName:  pfEthName,
			vfNumber:   vfNumber,
		})
	}

	return devices
}
