package main

import (
	"log/slog"
	"os"
	"path"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var contentKeys = map[string]string{"uuid": "key"}

func main() {
	logger := slog.Default()
	packs, err := parse(contentKeys)
	if err != nil {
		logger.Error("failed to parse resource packs", "err", err)
		return
	}

	listenConfig := minecraft.ListenConfig{
		ResourcePacks:        packs,
		StatusProvider:       util.NewStatusProvider("Spectrum Proxy", "Spectrum"),
		TexturePacksRequired: true,
	}
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("127.0.0.1:19133", ""), logger, nil, nil)
	if err := proxy.Listen(listenConfig); err != nil {
		return
	}

	for {
		_, _ = proxy.Accept()
	}
}

func parse(keys map[string]string) ([]*resource.Pack, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dir := path.Join(wd, "resource_packs")
	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			return nil, err
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var packs []*resource.Pack
	for _, entry := range entries {
		pack, err := resource.ReadPath(path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}

		if key, ok := keys[pack.UUID().String()]; ok {
			pack = pack.WithContentKey(key)
		}
		packs = append(packs, pack)
	}
	return packs, nil
}
