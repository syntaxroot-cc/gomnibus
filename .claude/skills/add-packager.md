# add-packager

Add a new package format (e.g. `.pkg`, `.msi`, `.apk`) to gomnibus.

## Usage

```
/add-packager <format>
```

Where `<format>` is one of: `pkg` (macOS), `msi` (Windows), `apk` (Alpine),
`snap`, `flatpak`, etc.

## Steps

1. Read existing packagers for structure:
   - `internal/packager/deb/deb.go`
   - `internal/packager/rpm/rpm.go`
   - `internal/packager/tar/tar.go`

2. Use the packager-author agent to implement the new packager.

3. Register it in `cmd/gomnibus/main.go`:
   ```go
   _ "github.com/syntaxroot-cc/gomnibus/internal/packager/<format>"
   ```

4. Write a unit test in `internal/packager/<format>/<format>_test.go`.

5. Document the required system tools (e.g. `pkgbuild` requires Xcode CLI tools).

## System tool requirements

| Format | Required tool       | Install command                  |
|--------|---------------------|----------------------------------|
| deb    | dpkg-deb            | `apt install dpkg`               |
| rpm    | rpmbuild            | `dnf install rpm-build`          |
| pkg    | pkgbuild, productbuild | `xcode-select --install`      |
| msi    | candle, light (WiX) | `choco install wixtoolset`       |
| apk    | abuild              | `apk add alpine-sdk`             |
