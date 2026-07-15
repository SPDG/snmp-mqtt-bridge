#!/usr/bin/env bash
set -euo pipefail

manifest="snmp-mqtt-bridge/config.yaml"
changelog="snmp-mqtt-bridge/CHANGELOG.md"

version=$(sed -n 's/^version: *"\([^"]*\)".*/\1/p' "${manifest}")
if [[ -z "${version}" ]]; then
    echo "Unable to read app version from ${manifest}" >&2
    exit 1
fi

version_pattern=${version//./\\.}
if ! grep -Eq "^## \\[?${version_pattern}\\]?([[:space:]]|$)" "${changelog}"; then
    echo "Missing changelog entry for app version ${version}" >&2
    exit 1
fi

if [[ "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
    tag_version=${GITHUB_REF_NAME#v}
    if [[ "${tag_version}" != "${version}" ]]; then
        echo "Tag ${GITHUB_REF_NAME} does not match app version ${version}" >&2
        exit 1
    fi
fi

echo "Changelog contains app version ${version}"
