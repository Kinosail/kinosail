#!/bin/sh
set -eu
previous=
input=
for argument do
    if [ "$previous" = -i ]; then
        input=$argument
        break
    fi
    previous=$argument
done
[ -n "$input" ] || exit 2
printf '%s\n' "$*" >> "${input%/*}/arguments"
case "$*" in
    *scale=160:90*) head -c 14400 /dev/zero ;;
    *) head -c 144 /dev/zero ;;
esac
