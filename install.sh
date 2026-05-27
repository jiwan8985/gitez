#!/usr/bin/env bash
# gez 설치 스크립트 (Mac / Linux)
# 사용법: bash install.sh

set -euo pipefail

BINARY="gez"
INSTALL_DIR="/usr/local/bin"

echo ""
echo "  🔧  gez 빌드 중..."
go build -ldflags "-s -w" -o "$BINARY" .

echo "  📦  $INSTALL_DIR/$BINARY 로 설치 중..."
if [ -w "$INSTALL_DIR" ]; then
    cp "$BINARY" "$INSTALL_DIR/$BINARY"
else
    sudo cp "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "  ✅  설치 완료!  gez 명령어를 어디서든 사용할 수 있습니다."
echo ""
echo "  시작하기:"
echo "    gez ws add        현재 폴더를 워크스페이스에 등록"
echo "    gez               대시보드 / 워크스페이스 보기"
echo "    gez -p <이름> s   다른 프로젝트 상태 확인"
echo ""
