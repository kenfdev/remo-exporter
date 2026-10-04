#!/bin/sh
# Sourced by the image scripts after tag is set. Docker tags cannot contain
# SemVer build metadata (+...), so reject it instead of silently rewriting it.
version=${tag#v}
channel=development
case "$version" in
	''|*[!a-zA-Z0-9_.-]*) echo 'Invalid image tag.' >&2; exit 2 ;;
esac
if [ "${#version}" -gt 128 ]; then
	echo 'Invalid image tag.' >&2
	exit 2
fi
case "$version" in
	[a-zA-Z0-9_]*) ;;
	*) echo 'Invalid image tag.' >&2; exit 2 ;;
esac

case "$tag" in
	v*|[0-9]*)
		number='(0|[1-9][0-9]*)'
		identifier='(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)'
		stable="$number\\.$number\\.$number"
		if printf '%s\n' "$version" | grep -Eq "^$stable$"; then
			channel=version
			case "$tag" in v*) channel=stable ;; esac
		elif printf '%s\n' "$version" | grep -Eq "^$stable-$identifier(\\.$identifier)*$"; then
			channel=prerelease
		else
			echo 'Invalid release version (expected SemVer without build metadata).' >&2
			exit 2
		fi
		;;
esac
