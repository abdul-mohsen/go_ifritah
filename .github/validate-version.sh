#!/usr/bin/env bash
set -euo pipefail

version="$(head -n 1 VERSION | sed 's/[[:space:]]//g')"
semver_regex='^v[0-9]+\.[0-9]+\.[0-9]+$'

if [[ ! "${version}" =~ ${semver_regex} ]]; then
  echo "VERSION must match ${semver_regex}; got '${version}'" >&2
  exit 1
fi

if [[ -z "${GITHUB_REPOSITORY:-}" ]]; then
  echo "GITHUB_REPOSITORY is not set; skipped latest GitHub Release comparison"
  echo "VERSION=${version}"
  exit 0
fi

headers=(
  -H "Accept: application/vnd.github+json"
  -H "X-GitHub-Api-Version: 2022-11-28"
)

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  headers+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

response_file="$(mktemp)"
status="$(curl -sS -L -w '%{http_code}' -o "${response_file}" "${headers[@]}" \
  "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/latest" || true)"

if [[ "${status}" == "404" ]]; then
  echo "No GitHub Release exists yet; VERSION=${version} is valid"
  exit 0
fi

if [[ "${status}" != "200" ]]; then
  echo "Failed to read latest GitHub Release for ${GITHUB_REPOSITORY}; HTTP ${status}" >&2
  cat "${response_file}" >&2
  exit 1
fi

latest="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "${response_file}" | head -n 1)"

if [[ ! "${latest}" =~ ${semver_regex} ]]; then
  echo "Latest GitHub Release tag must match ${semver_regex}; got '${latest}'" >&2
  exit 1
fi

if [[ "${version}" != "${latest}" ]]; then
  lowest="$(printf '%s\n%s\n' "${version}" "${latest}" | sort -V | head -n 1)"
  if [[ "${lowest}" == "${version}" ]]; then
    echo "VERSION ${version} is lower than latest GitHub Release ${latest}" >&2
    exit 1
  fi
fi

echo "VERSION=${version} is valid; latest release=${latest}"
