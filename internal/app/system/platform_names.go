package system

// CLI 产物的平台标识，与发布包命名约定（<lang>-<os>-<arch>）一致。
const (
	osWindows = "windows"
	osDarwin  = "darwin"
	osLinux   = "linux"

	archAMD64 = "amd64"
	archARM64 = "arm64"

	// platformAny 表示与平台无关的产物（Python/Java/C#）。
	platformAny = "any-any"
)
