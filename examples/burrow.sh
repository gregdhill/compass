#!/usr/bin/env bash

export CHAIN_NODES=4
keysTemplate=$(mktemp)
valsTemplate=$(mktemp)
genSpec=$(mktemp)
genesis=$(mktemp)
keys=$(mktemp -d)

cat >$keysTemplate <<EOF
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: << .Config.ChainName >>-keys
data:
  <<- \$keys:=.Keys ->>
  <<- range .Keys ->>
    <<- if index \$keys .Address >>
    << .Address >>.json: << base64 (index \$keys .Address).KeyJSON >>
    <<- end ->>
  <<- end ->>
  <<- range .Validators ->>
    <<- if index \$keys .NodeAddress >>
    nodekey-<< .Name >>: << base64 (index \$keys .NodeAddress).KeyJSON >>
    <<- end ->>
  <<- end ->>
EOF

cat >$valsTemplate <<EOF
chain:
  nodes: $CHAIN_NODES

validatorAddresses:
  <<- range .Config.Validators >>
  << .Name >>:
    Address: << .Address ->>
    <<if .NodeAddress >>
    NodeAddress: << .NodeAddress >>
    <<- end ->>
  <<- end >>
EOF

burrow spec \
  --toml \
  --validator-accounts=$CHAIN_NODES > $genSpec

burrow configure \
  --generate-node-keys \
  --chain-name=$chain_release \
  --keysdir=$keys \
  --genesis-spec=$genSpec \
  --config-template-in=$keysTemplate \
  --config-out=chain-info.yaml \
  --config-template-in=$valsTemplate \
  --config-out=initialize.yaml \
  --separate-genesis-doc=$genesis >/dev/null

cat >>chain-info.yaml <<EOF

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${chain_release}-genesis
data:
  genesis.json: |
    `cat $genesis | jq -rc .`
EOF

rm $keysTemplate
rm $valsTemplate
rm $genSpec
rm $genesis
rm -r $keys

kubectl apply -f chain-info.yaml