package acserver

import "testing"

type checksumSanitiseTest struct {
	filePath   string
	shouldPass bool
}

func TestSanitiseChecksums(t *testing.T) {
	checksums := []checksumSanitiseTest{
		{
			"\\\\blueflash\\downloads\\some.torrent",
			false,
		},
		{
			"content/cache/assists.ini",
			true,
		},
		{
			`C:Projects\apilibrary\apilibrary.sln`,
			false,
		},
		{
			"2018\\January.xlsx",
			true,
		},
		{
			`\Program Files\Custom Utilities\StringFinder.exe`,
			false,
		},
		{
			`\\?\Volume{b75e2c83-0000-0000-0000-602f00000000}\Test\Foo.txt`,
			false,
		},
		{
			"C:\\foo\bar\baz",
			false,
		},
		{
			"/app/data/db.foo",
			false,
		},
		{
			"/foo/bar/baz",
			false,
		},
		{
			"content/../../../some/thing.exe",
			false,
		},
		{
			"../content/../",
			false,
		},
		{
			"..",
			false,
		},
		{
			"X:\\Users\\Callum\\Documents\\secret.txt",
			false,
		},
		{
			"content/cars/ok/data.acd",
			true,
		},
		{
			"apps/python/helicorsa/helicorsa.py",
			true,
		},
	}

	for _, checksum := range checksums {
		t.Run(checksum.filePath, func(t *testing.T) {
			pass := sanitiseChecksumPath(checksum.filePath)

			if pass != checksum.shouldPass {
				t.Errorf("Failed santising checksum: %s", checksum.filePath)
			}
		})
	}
}
