package main

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"

	"justapengu.in/acsm/pkg/license"
)

const (
	privateKey = "FD7YCAYBAEFXA22DN5XHIYLJNZSXEAP7QIAACAQBANIHKYQBBIAACAKEAH7YIAAAAAFP7AYFAEBP7BQAAAAP7GP7QIAWCBCGEZB737JDRP65X3MFLMXEOW3MT56YAV2KMIDMRVOMP2T2JMEC3NR7AIAUNPY5ULLLGKEZZSII3BKFHRMPYQVRZHANQXEZTHPSQSMYXW4PIQXAGZ64S2YA4EB4ORF3HOR74R3KTMELA2UCK5V6JC5KC4DTAEYQFG33LOODUSVMFGFGJWIKDRQF7WW3UDPAS6DP6TZBEW743G5BCOGAMOHZMGJIO75KVQRZ77GTQZM3N4AA===="
)

func main() {
	l, encoded, err := license.GenerateLicense(privateKey, "support@emperorservers.com", time.Time{}, license.ManagerTypeACSM)

	if err != nil {
		panic(err)
	}

	spew.Dump(l)

	fmt.Println(encoded)
}
