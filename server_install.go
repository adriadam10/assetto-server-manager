package acsm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// assettoServerSteamID is the ID of the server on steam.
const assettoServerSteamID = "302550"

var (
	ErrNoSteamCMD = errors.New("servermanager: steamcmd was not found in $PATH")

	// ServerInstallPath is where the assetto corsa server should be/is installed
	ServerInstallPath = "assetto"

	ServerConfigPath = "cfg"
)

func SetAssettoInstallPath(installPath string) {
	if !filepath.IsAbs(installPath) {
		wd, err := os.Getwd()

		if err == nil {
			ServerInstallPath = filepath.Join(wd, installPath)
		} else {
			panic("unable to get working directory. can't install server")
		}
	} else {
		ServerInstallPath = installPath
	}
}

func IsAssettoInstalled() bool {
	_, err := os.Stat(filepath.Join(ServerInstallPath, "system"))

	return err == nil
}

// InstallAssettoCorsaServerWithSteamCMD takes a steam login and password and runs steamcmd to install the assetto server.
// If the "ServerInstallPath" exists, this function will exit without installing - unless force == true.
func InstallAssettoCorsaServerWithSteamCMD(login, password string, force bool) error {
	_, err := os.Stat(filepath.Join(ServerInstallPath, "system"))

	if err != nil && !os.IsNotExist(err) {
		return err
	} else if !force && !os.IsNotExist(err) {
		return nil // server is installed
	}

	logrus.Infof("Attempting to install the Assetto Corsa Server (steamid: %s) to %s", assettoServerSteamID, ServerInstallPath)

	commandToUse := "steamcmd.sh"

	if !isCommandAvailable(commandToUse) {
		if isCommandAvailable("steamcmd") {
			logrus.Infof("WARNING using steamcmd instead of steamcmd.sh. You must have run steamcmd before using this tool or Assetto Corsa Server will not install correctly.")
			commandToUse = "steamcmd"
		} else {
			return ErrNoSteamCMD
		}
	}

	cmd := exec.Command(commandToUse,
		"+@sSteamCmdForcePlatformType windows",
		fmt.Sprintf("+login %s %s", login, password),
		"+force_install_dir "+ServerInstallPath,
		"+app_update "+assettoServerSteamID,
		"+quit",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	if err != nil {
		return err
	}

	// create default skins
	for _, f := range defaultSkinsLayout {
		err := os.MkdirAll(filepath.Join(ServerInstallPath, filepath.FromSlash(f)), 0755)

		if err != nil {
			return err
		}
	}

	return nil
}

func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func InstallBareBonesAssettoCorsaServer() error {
	if IsAssettoInstalled() {
		return nil
	}

	systemPath := filepath.Join(ServerInstallPath, "system", "data")

	installPaths := []string{
		systemPath,
		filepath.Join(ServerInstallPath, "cfg"),
		filepath.Join(ServerInstallPath, "results"),
		filepath.Join(ServerInstallPath, "content", "cars"),
		filepath.Join(ServerInstallPath, "content", "tracks"),
		filepath.Join(ServerInstallPath, "content", "weather"),
		filepath.Join(ServerInstallPath, "setups"),
		filepath.Join(ServerInstallPath, "logs"),
		filepath.Join(ServerInstallPath, "manager"),
	}

	for _, installPath := range installPaths {
		if err := os.MkdirAll(installPath, 0755); err != nil {
			return err
		}
	}

	surfaces, err := os.Create(filepath.Join(systemPath, "surfaces.ini"))

	if err != nil {
		return err
	}

	defer surfaces.Close()

	_, err = surfaces.Write(systemSurfacesFile)

	if err != nil {
		return err
	}

	for _, f := range defaultSkinsLayout {
		err := os.MkdirAll(filepath.Join(ServerInstallPath, filepath.FromSlash(f)), 0755)

		if err != nil {
			return err
		}
	}

	tyres, err := os.Create(filepath.Join(ServerInstallPath, "manager", "ks_tyres.ini"))

	if err != nil {
		return err
	}

	defer tyres.Close()

	_, err = tyres.WriteString(defaultKSTyresIni)

	if err != nil {
		return err
	}

	return nil
}
