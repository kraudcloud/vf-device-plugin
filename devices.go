package main

import (
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

	iommu := regexp.MustCompile(`\/(\d+)$`)

	var devices []vfioDevice

	path := "/sys/class/net/"
	list, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range list {

		pfEthName := file.Name()

		fullpath := path + "/" + pfEthName

		path := "/sys/class/net/" + pfEthName + "/device/"
		list, err := os.ReadDir(path)
		if err != nil {
			log.Println(err)
			continue
		}

		for _, file := range list {

			if !strings.HasPrefix(file.Name(), "virtfn") {
				continue
			}

			vfNumber := strings.TrimPrefix(file.Name(), "virtfn")

			fullpath := fullpath + "/device/" + file.Name()

			st, err := os.Readlink(fullpath)
			if err != nil {
				log.Println(err)
				continue
			}

			pcieName := filepath.Base(st)

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

			log.Print("Found PCI device " + pcieName)
			log.Print("Vendor " + vendor)
			log.Print("Device " + device)
			log.Print("IOMMU Group " + dest)
			log.Print("PF ethernet device name " + pfEthName)

			devices = append(devices, vfioDevice{
				pciName:    pcieName,
				vendorId:   vendor,
				deviceId:   device,
				iommuGroup: dest,
				pfEthName:  pfEthName,
				vfNumber:   vfNumber,
			})
		}

	}

	return devices
}
