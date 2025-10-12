$PROTO_VER="0.17.0"
$PROTO_IMAGE_NAME="ghcr.io/cosmos/proto-builder:$PROTO_VER"

docker run --rm -v ${PWD}:/workspace -w /workspace $PROTO_IMAGE_NAME sh ./proto/scripts/protocgen.sh