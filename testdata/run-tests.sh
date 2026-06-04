#!/usr/bin/env bash
#
# Regression harness for raidstat, driven by the mock vendor tools in testdata/.
# Build first, then run from anywhere:
#
#   make && ./testdata/run-tests.sh
#
# It points build/config.json at the mock tools, exercises the multi-vendor
# discovery/status flow exactly as Zabbix would, and asserts the output. The
# real config.json is restored on exit.

set -u
cd "$(dirname "$0")/.." || exit 1 # repo root: mock tools cat testdata/<vendor>/*.txt relative to CWD

BIN=./build/raidstat
if [ ! -x "$BIN" ]; then
	echo "raidstat not built - run 'make' first"
	exit 1
fi

cat >build/config.json <<EOF
{ "vendors": {
  "megacli": "$(pwd)/testdata/megacli.sh",
  "mdstat":  "$(pwd)/testdata/mdstat.sh"
} }
EOF
trap 'cp config.json build/config.json' EXIT # restore the shipped config

fail=0

assert_eq() { # desc expected actual
	if [ "$2" = "$3" ]; then
		echo "ok   - $1"
	else
		echo "FAIL - $1"
		echo "       expected: $2"
		echo "       actual:   $3"
		fail=1
	fi
}

assert_has() { # desc needle haystack
	case "$3" in
	*"$2"*) echo "ok   - $1" ;;
	*)
		echo "FAIL - $1"
		echo "       missing: $2"
		fail=1
		;;
	esac
}

assert_absent() { # desc needle haystack
	case "$3" in
	*"$2"*)
		echo "FAIL - $1"
		echo "       unexpected: $2"
		fail=1
		;;
	*) echo "ok   - $1" ;;
	esac
}

echo "# mixed-vendor discovery (megacli + mdstat on one host)"
ct=$("$BIN" --vendor "megacli,mdstat" -d ct)
assert_eq "controllers tagged by vendor (collision-free)" \
	'{"data":[{"{#VENDOR}":"megacli","{#CT_ID}":"0"},{"{#VENDOR}":"mdstat","{#CT_ID}":"0"}]}' \
	"$ct"

ld=$("$BIN" --vendor "megacli,mdstat" -d ld)
assert_has "ld: megacli array" '"{#VENDOR}":"megacli","{#CT_ID}":"0","{#LD_ID}":"2"' "$ld"
assert_has "ld: mdstat md0" '"{#VENDOR}":"mdstat","{#CT_ID}":"0","{#LD_ID}":"md0"' "$ld"
assert_has "ld: IMSM volume present" '"{#LD_ID}":"md/Volume0_0"' "$ld"
assert_absent "ld: IMSM container excluded" '"{#LD_ID}":"md/imsm0"' "$ld"

pd=$("$BIN" --vendor "megacli,mdstat" -d pd)
assert_has "pd: megacli drive" '"{#VENDOR}":"megacli","{#CT_ID}":"0","{#PD_ID}":"252:0"' "$pd"
assert_has "pd: mdstat NVMe member" '"{#PD_ID}":"md/2:nvme0n1p1"' "$pd"
assert_has "pd: IMSM volume member" '"{#PD_ID}":"md/Volume0_0:sda"' "$pd"
assert_absent "pd: IMSM container not a drive" 'imsm0' "$pd"

echo
echo "# per-vendor status (as the template's UserParameters invoke it)"
assert_eq "megacli controller status" \
	'{"status":"OK","model":"LSI MegaRAID SAS 9261-8i","batterystatus":"OK"}' \
	"$("$BIN" -v megacli -s ct,0)"
assert_eq "mdstat md1 logical drive (degraded)" \
	'{"status":"clean, degraded","size":"2095104 (2046.00 MiB 2145.39 MB)"}' \
	"$("$BIN" -v mdstat -s ld,0,md1)"
assert_eq "mdstat IMSM volume status (State active)" \
	'{"status":"OK","size":"222715904 (212.40 GiB 228.06 GB)"}' \
	"$("$BIN" -v mdstat -s ld,0,md/Volume0_0)"

echo
echo "# guard: status must refuse a multi-vendor request"
if "$BIN" --vendor "megacli,mdstat" -s ct,0 >/dev/null 2>&1; then
	echo "FAIL - status accepted multiple vendors"
	fail=1
else
	echo "ok   - status rejects multiple vendors"
fi

echo
if [ "$fail" -eq 0 ]; then
	echo "ALL PASS"
else
	echo "FAILURES"
	exit 1
fi
