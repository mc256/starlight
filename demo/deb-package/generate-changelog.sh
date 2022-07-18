#!/bin/bash
VERSION_FORMAT='^v\?[0-9.-]\+'
function logentry() {
    local previous=$1
    local version=$2
    local version_number=`echo $2 | sed 's/v//g'`
    local version_number=`[[ -z "$version_number" ]] && echo "0.0.0" || echo $version_number`
    echo "starlight-snapshotter ($version_number) unstable; urgency=low"
    echo
    git --no-pager log --format="  * %s" $previous${previous:+..}$version
    echo
    git --no-pager log --format=" -- %an <%ae>  %aD" -n 1 $version
    echo
}

git tag --sort "-version:refname" | grep "$VERSION_FORMAT" | (
    read version; while read previous; do
        logentry $previous $version
        version="$previous"
    done
    logentry "" $version
)