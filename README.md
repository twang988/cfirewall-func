# cfirewall-func

## debug
```
DIR=./hostdev-data
FN_CFG=${DIR}/func-cfg-hostdev.yaml
kpt fn source ${DIR} --fn-config=${FN_CFG} | go run main.go
```