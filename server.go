// Copyright (c) 2019, Arm Ltd

package main

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	api "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"
	"unicode"
)

type vfioDevicePlugin struct {
	devs         []*api.Device
	socket       string
	resourceName string
	iommuGroup   string
	pcieName     string

	stop   chan interface{}
	server *grpc.Server
}

func (m *vfioDevicePlugin) cleanup() error {
	err := os.Remove(m.socket)
	if err == nil {
		log.Print("Removing file ", m.socket)
		return nil
	}

	if os.IsNotExist(err) {
		return nil
	}

	return err
}

func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		unixSocketPath,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return conn, nil
}

func NewDevicePlugin(ii *vfInstance) *vfioDevicePlugin {
	var devices []*api.Device

	devices = append(devices, &api.Device{
		ID:     ii.iommuGroup,
		Health: api.Healthy,
	})

	return &vfioDevicePlugin{
		devs:         devices,
		socket:       ii.socketName,
		resourceName: ii.resourceName,
		iommuGroup:   ii.iommuGroup,
		pcieName:     ii.pcieName,
		stop:         make(chan interface{}),
	}
}

func (m *vfioDevicePlugin) Serve() error {
	err := m.Start()
	if err != nil {
		log.Print("Could not start device plugin: ", err)
		return err
	}
	log.Print("Starting to serve on ", m.socket)

	err = m.Register(api.KubeletSocket, m.resourceName)
	if err != nil {
		log.Print("Could not register device plugin: ", err)
		m.Stop()
		return err
	}
	log.Print("Registered device plugin with Kubelet")

	return nil
}

func (m *vfioDevicePlugin) Register(kubeletEndpoint, resourceName string) error {
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := api.NewRegistrationClient(conn)
	reqt := &api.RegisterRequest{
		Version:      api.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}

	return nil
}

func (m *vfioDevicePlugin) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	api.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socket, 60*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

func (m *vfioDevicePlugin) Stop() error {
	log.Printf("Stopping server with socket ", m.socket)
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil
	close(m.stop)
	log.Print("Server stopped with socket ", m.socket)

	return m.cleanup()
}

func (m *vfioDevicePlugin) ListAndWatch(e *api.Empty, s api.DevicePlugin_ListAndWatchServer) error {
	s.Send(&api.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		}
	}
}

func (m *vfioDevicePlugin) Allocate(ctx context.Context, reqs *api.AllocateRequest) (*api.AllocateResponse, error) {
	responses := api.AllocateResponse{}

	for _, req := range reqs.ContainerRequests {
		var devices []*api.DeviceSpec

		for _, id := range req.DevicesIDs {
			log.Print("Allocating IOMMU Group " + id)

			devices = append(devices, &api.DeviceSpec{
				ContainerPath: "/dev/vfio/vfio",
				HostPath:      "/dev/vfio/vfio",
				Permissions:   "mrw",
			})

			devices = append(devices, &api.DeviceSpec{
				ContainerPath: "/dev/vfio/" + id,
				HostPath:      "/dev/vfio/" + id,
				Permissions:   "mrw",
			})

			devices = append(devices, &api.DeviceSpec{
				ContainerPath: "/sys/bus/pci/devices/" + m.pcieName,
				HostPath:      "/sys/bus/pci/devices/" + m.pcieName,
				Permissions:   "mrw",
			})

			devices = append(devices, &api.DeviceSpec{
				ContainerPath: "/sys/bus/pci/drivers/vfio-pci/" + m.pcieName,
				HostPath:      "/sys/bus/pci/drivers/vfio-pci/" + m.pcieName,
				Permissions:   "mrw",
			})

			devices = append(devices, &api.DeviceSpec{
				ContainerPath: "/sys/kernel/iommu_groups/" + id,
				HostPath:      "/sys/kernel/iommu_groups/" + id,
				Permissions:   "mrw",
			})

			split := strings.Split(m.pcieName, ":")
			pp2 := strings.Join(split[:len(split)-1], ":")

			devices = append(devices, &api.DeviceSpec{
				ContainerPath: "/sys/devices/pci" + pp2,
				HostPath:      "/sys/devices/pci" + pp2,
				Permissions:   "mrw",
			})
		}

		envKey1 := cleanEnvName1("PCI_RESOURCE_" + m.resourceName)
		envKey2 := cleanEnvName2("PCI_RESOURCE_" + m.resourceName)
		response := api.ContainerAllocateResponse{
			Devices: devices,
			Envs: map[string]string{
				envKey1: m.pcieName,
				envKey2: m.pcieName,
			},
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	return &responses, nil
}

func (m *vfioDevicePlugin) PreStartContainer(context.Context, *api.PreStartContainerRequest) (*api.PreStartContainerResponse, error) {
	return &api.PreStartContainerResponse{}, nil
}

func (m *vfioDevicePlugin) GetPreferredAllocation(context.Context, *api.PreferredAllocationRequest) (*api.PreferredAllocationResponse, error) {
	return &api.PreferredAllocationResponse{}, nil
}

func (m *vfioDevicePlugin) GetDevicePluginOptions(context.Context, *api.Empty) (*api.DevicePluginOptions, error) {
	return &api.DevicePluginOptions{}, nil
}

func cleanEnvName1(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteString("_")
		}
	}
	return b.String()
}

func cleanEnvName2(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' ||

			// kubevirt does not replace - with _ when looking for the env D:
			r == '_' || r == '-' {

			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteString("_")
		}
	}
	return b.String()
}
