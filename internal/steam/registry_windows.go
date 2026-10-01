//go:build windows

package steam

import "golang.org/x/sys/windows/registry"

func registrySteamPath() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue("SteamPath")
	if err != nil {
		return ""
	}
	return v
}

func registryActiveUser() (uint32, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam\ActiveProcess`, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("ActiveUser")
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}
