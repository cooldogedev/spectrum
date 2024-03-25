package main

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"github.com/spectrum-proxy/spectrum"
	"github.com/spectrum-proxy/spectrum/server"
	"os"
)

func main() {
	logger := logrus.New()
	packs, err := parse(map[string]string{
		"uuid": "key",
	})
	if err != nil {
		logger.Errorf("Failed to parse resource packs: %v", err)
		return
	}

	listenConfig := minecraft.ListenConfig{
		StatusProvider:       spectrum.NewStatusProvider("Spectrum Proxy"),
		TexturePacksRequired: true,
		ResourcePacks:        packs,
	}
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery(":19133"), logger, nil)
	if err := proxy.Listen(listenConfig); err != nil {
		logger.Errorf("Failed to listen on proxy: %v", err)
		return
	}

	for {
		s, err := proxy.Accept()
		if err != nil {
			if s != nil {
				s.Disconnect(err.Error())
			}
			logger.Errorf("Failed to accept session: %v", err)
		}
	}
}

func parse(keys map[string]string) ([]*resource.Pack, error) {
	path := "PATH_TO_RESOURCE_PACKS"
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var packs []*resource.Pack

	for _, entry := range entries {
		pack, err := resource.ReadPath(path + entry.Name())
		if err != nil {
			return nil, err
		}

		key, ok := keys[pack.UUID()]
		if ok {
			pack.WithContentKey(key)
		}

		packs = append(packs, pack)
	}

	return packs, nil
}
