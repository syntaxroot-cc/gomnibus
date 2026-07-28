---
name: gomnibus-dsl-reference
description: Complete YAML DSL reference for software and project definitions
type: project
---

# gomnibus YAML DSL Reference

## Software definition (`config/software/<name>.yaml`)

```yaml
name: <string>                 # component identifier (required)
default_version: "<string>"    # default version; overridable per-project
license: <SPDX-ID>             # e.g. MIT, Apache-2.0, GPL-2.0-only
license_file: LICENSE          # relative path inside source tree

source:                        # one of url/git/path/s3_bucket required
  url: "https://..."
  git: "https://github.com/..."
  path: "/local/path"
  s3_bucket: "my-bucket"
  s3_key: "prefix/archive.tar.gz"
  md5: "<32-hex>"              # use sha256 when possible
  sha1: "<40-hex>"
  sha256: "<64-hex>"           # preferred
  sha512: "<128-hex>"
  branch: main                 # git only
  tag: v1.2.3                  # git only
  commit: abc1234              # git only

relative_path: "<name>-<version>"  # extracted/cloned directory name

dependencies:
  - other-software

whitelist_files:               # regex patterns for health check exemptions
  - "libpthread\\.so"

skip_healthcheck: false        # skip ldd/otool check for this component

env:                           # environment variables for all build steps
  CFLAGS: "-O2"

build:
  # Shell command (via sh -c)
  - command: "./bootstrap.sh"

  # make with optional targets
  - make: []                   # bare make
  - make:
      - install

  # ./configure with flags (--prefix always injected)
  - configure:
      - "--disable-static"
      - "--enable-shared"

  # cmake
  - cmake:
      - "-DBUILD_SHARED_LIBS=ON"

  # go build
  - go:
      - build
      - "-ldflags=-s -w"
      - "-o"
      - "${install_dir}/bin/foo"
      - "./cmd/foo"

  # file ops
  - mkdir: "${install_dir}/etc/foo"
  - copy:
      src: "config.yaml.example"
      dst: "${install_dir}/etc/foo/config.yaml"
  - move:
      src: "old/path"
      dst: "new/path"
  - link:
      target: "${install_dir}/bin/foo"
      link: "${install_dir}/bin/foo2"
  - delete: "${install_dir}/share/man"

  # patch
  - patch:
      source: "my.patch"
      plevel: 1

  # per-step env override
  - command: "make"
    env:
      MAKEFLAGS: "-j4"
    work_dir: "build"

# Path tokens available in build steps:
#   ${install_dir}  — project's install_dir
#   ${src_dir}      — where the source was fetched/cloned
#   ${build_dir}    — temporary build scratch dir

versions:
  - version: "1.2.0"
    source:
      url: "https://..."
      sha256: "..."
    build:                     # optional: override build for this version
      - configure: []
      - make: []
      - make: [install]
```

## Project definition (`config/projects/<name>.yaml`)

```yaml
name: myproject
friendly_name: "My Project"
maintainer: "Acme <pkg@acme.com>"
homepage: "https://acme.com"
description: "Full-stack installer for My Project"
install_dir: /opt/myproject
build_version: "1.0.0"
build_iteration: 1
license: Apache-2.0

dependencies:               # ordered list; gomnibus resolves full dep tree
  - zlib
  - openssl
  - myproject

overrides:                  # pin a software to a specific version
  - name: openssl
    version: "3.3.1"

packages:
  - type: deb               # deb, rpm, pkg, msi, tar
  - type: rpm
  - type: tar
    options:
      compression: xz

compress:
  type: xz                  # xz, gzip, bzip2, zstd
  level: 9

runtime_dependencies:       # injected into package metadata
  - "libc6 (>= 2.17)"

conflicts:
  - old-myproject

replaces:
  - old-myproject

exclude_files:
  - "**/*.a"                # strip static libs from the package
  - "**/man/**"

extra_package_files:
  - source: "scripts/postinst"
    destination: "DEBIAN/postinst"
    mode: "0755"
```

## Global config (`gomnibus.yaml`)

```yaml
base_dir: ~/.gomnibus/build
cache_dir: ~/.gomnibus/cache
use_git_caching: true
use_s3_caching: false
s3_bucket: ""
s3_region: us-east-1
s3_access_key: ""
s3_secret_key: ""
s3_profile: ""
s3_iam_role_arn: ""
append_timestamp: false
workers: 4
log_level: info
software_dirs:
  - config/software
```
