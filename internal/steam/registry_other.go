//go:build !windows

package steam

func registrySteamPath() string { return "" }

func registryActiveUser() (uint32, bool) { return 0, false }
