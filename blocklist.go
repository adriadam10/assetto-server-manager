package acsm

import (
	"encoding/json"
	"os"
	"path/filepath"

	"justapengu.in/acsm/internal/acserver"
)

type BlockListManager struct{}

func NewBlockListManager() *BlockListManager {
	return &BlockListManager{}
}

func (bl *BlockListManager) LoadBlockList() ([]string, error) {
	var blockList []string

	f, err := os.Open(filepath.Join(ServerInstallPath, acserver.BlockListFileName))

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		defer f.Close()

		if err := json.NewDecoder(f).Decode(&blockList); err != nil {
			return nil, err
		}
	}

	return blockList, nil
}

func (bl *BlockListManager) AddToBlockList(guid string) error {
	guid = CleanGUID(guid)

	blockList, err := bl.LoadBlockList()

	if err != nil {
		return err
	}

	guidExists := false

	for _, existingGUID := range blockList {
		if guid == existingGUID {
			guidExists = true
			break
		}
	}

	if guidExists {
		return nil
	}

	blockList = append(blockList, guid)

	return bl.UpdateBlockList(blockList)
}

func (bl *BlockListManager) RemoveFromBlockList(guid string) error {
	blockList, err := bl.LoadBlockList()

	if err != nil {
		return err
	}

	toRemove := -1

	for i, existingGUID := range blockList {
		if existingGUID == guid {
			toRemove = i
			break
		}
	}

	if toRemove < 0 {
		return nil
	}

	blockList = append(blockList[:toRemove], blockList[toRemove+1:]...)

	return bl.UpdateBlockList(blockList)
}

func (bl *BlockListManager) UpdateBlockList(guids []string) error {
	f, err := os.Create(filepath.Join(ServerInstallPath, acserver.BlockListFileName))

	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(guids)
}
