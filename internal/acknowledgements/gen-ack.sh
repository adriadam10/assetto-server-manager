#!/bin/bash
tmp_dir=$(mktemp -d -t ci-XXXXXXXXXX)
OUT=$tmp_dir/ACKNOWLEDGEMENTS.txt

pushd () {
    command pushd "$@" > /dev/null
}

popd () {
    command popd "$@" > /dev/null
}


licenses=(
	"license"
	"license.txt"
	"license.md"
)

FINDB=find
SEDB=sed

if [ $(uname -s) == "Darwin" ]; then
    FINDB=gfind
    SEDB=gsed
fi

echo -e "Portions of Assetto Corsa Server Manager's development were aided and influenced by the 'actools' library (https://github.com/gro-ove/actools).\nUse of this software is governed by the terms of the license below:\n\n" >>$OUT
cat actools/LICENSE.txt >>$OUT
echo -e "\n\n----------\n\n" >>$OUT

pushd ../../
  go mod vendor

  pushd vendor/
    for LICENSE in ${licenses[@]}; do
        for i in $( $FINDB -iname $LICENSE | sort ); do
            NAME=$(echo $i | rev |cut -d'/' -f 2|rev)
            echo -e "Assetto Corsa Server Manager uses the '$NAME' library. Use of this software is governed by the terms of the license below:\n\n" >>$OUT
            cat $i >>$OUT
            echo -e "\n\n----------\n\n" >>$OUT
        done
    done
  popd

  rm -rf vendor


  pushd cmd/server-manager/typescript/node_modules
    for LICENSE in ${licenses[@]}; do
        for i in $( $FINDB -iname $LICENSE | sort ); do
            NAME=$(echo $i | rev |cut -d'/' -f 2|rev)
            echo -e "Assetto Corsa Server Manager uses the '$NAME' library. Use of this software is governed by the terms of the license below:\n\n" >>$OUT
            cat $i >>$OUT
            echo -e "\n\n----------\n\n" >>$OUT
        done
    done
  popd
popd

$SEDB -i 's/`/`\+"`"\+`/g' $OUT

echo "package acknowledgements" >acknowledgements.go
echo "" >>acknowledgements.go
echo "const Acknowledgements = \`" >>acknowledgements.go
cat $OUT >>acknowledgements.go
echo "\`" >>acknowledgements.go
echo "">>acknowledgements.go

rm $OUT
