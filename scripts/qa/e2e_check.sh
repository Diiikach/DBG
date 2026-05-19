#!/usr/bin/env bash
# End-to-end smoke test для проверки соответствия сервиса ТЗ.
# Не требует наличия фронта — стучится напрямую в API.
#
# Использование: API=http://localhost:8081 bash scripts/qa/e2e_check.sh
STATUS=0

API="${API:-http://localhost:8081}"
TS=$(date +%s)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0
FAILED_TESTS=()

# -------------------- helpers --------------------

assert_eq() {
    local label="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "  [OK]  $label: $actual"
        PASS=$((PASS+1))
    else
        echo "  [FAIL] $label: expected=$expected actual=$actual"
        FAIL=$((FAIL+1))
        FAILED_TESTS+=("$label")
    fi
}

assert_in() {
    local label="$1" needle="$2" hay="$3"
    if [[ "$hay" == *"$needle"* ]]; then
        echo "  [OK]  $label: contains '$needle'"
        PASS=$((PASS+1))
    else
        echo "  [FAIL] $label: '$needle' not found in: ${hay:0:200}"
        FAIL=$((FAIL+1))
        FAILED_TESTS+=("$label")
    fi
}

req() {
    # req METHOD URL [token] [json-body|@file]  -> stdout: body; writes status to $TMP/status
    local method="$1" url="$2" token="${3:-}" body="${4:-}"
    local out="$TMP/resp"
    local args=(-sS -o "$out" -w "%{http_code}" -X "$method" "$API$url")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
    if [ -n "$body" ]; then
        if [ "${body:0:1}" = "@" ]; then
            args+=(--data-binary "$body" -H "Content-Type: application/json")
        else
            args+=(-H "Content-Type: application/json" -d "$body")
        fi
    fi
    curl "${args[@]}" > "$TMP/status"
    cat "$out"
}
status() { cat "$TMP/status"; }

upload_file() {
    local url="$1" token="$2"; shift 2
    local out="$TMP/resp"
    local args=(-sS -o "$out" -w "%{http_code}" -X POST "$API$url" -H "Authorization: Bearer $token")
    for kv in "$@"; do
        local k="${kv%%=*}" v="${kv#*=}"
        args+=(-F "$k=@$v")
    done
    curl "${args[@]}" > "$TMP/status"
    cat "$out"
}

jget() { python3 -c "import sys,json; d=json.load(sys.stdin); ks='$1'.split('.'); v=d
for k in ks:
    v = v[int(k)] if isinstance(v,list) else v.get(k) if v is not None else None
print('' if v is None else v)"; }

section() { echo; echo "============================================================"; echo "## $1"; echo "============================================================"; }

# -------------------- 0. Health --------------------

section "0. Health"
body=$(req GET /healthz); assert_eq "healthz" 200 "$(status)"

# -------------------- 1. Аутентификация (ТЗ 4.1.1 п.1) --------------------

section "1. Аутентификация (ТЗ 4.1.1 п.1)"
U1="qa_a_${TS}"
U2="qa_b_${TS}"

# 1.1 Регистрация без email — должна вернуть 422 (ТЗ требует email).
body=$(req POST /api/auth/register "" "{\"username\":\"noemail_$TS\",\"password\":\"secret123\"}")
assert_eq "register without email -> 422" 422 "$(status)"

# 1.2 Регистрация с битым email — 422.
body=$(req POST /api/auth/register "" "{\"username\":\"bademail_$TS\",\"password\":\"secret123\",\"email\":\"not-an-email\"}")
assert_eq "register with bad email -> 422" 422 "$(status)"

# 1.3 Регистрация с коротким паролем — 422.
body=$(req POST /api/auth/register "" "{\"username\":\"shortpw_$TS\",\"password\":\"123\",\"email\":\"shortpw_$TS@ex.com\"}")
assert_eq "register with short password -> 422" 422 "$(status)"

# 1.4 Корректная регистрация U1.
body=$(req POST /api/auth/register "" "{\"username\":\"$U1\",\"password\":\"secret123\",\"email\":\"$U1@ex.com\"}")
assert_eq "register U1 -> 201" 201 "$(status)"
T1=$(echo "$body" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
UID1=$(echo "$body" | python3 -c 'import sys,json;print(json.load(sys.stdin)["user"]["user_id"])')
assert_in "U1 email returned" "$U1@ex.com" "$body"

# 1.5 Повторная регистрация — 409.
body=$(req POST /api/auth/register "" "{\"username\":\"$U1\",\"password\":\"secret123\",\"email\":\"$U1@ex.com\"}")
assert_eq "register duplicate -> 409" 409 "$(status)"

# 1.6 Регистрация U2.
body=$(req POST /api/auth/register "" "{\"username\":\"$U2\",\"password\":\"secret123\",\"email\":\"$U2@ex.com\"}")
assert_eq "register U2 -> 201" 201 "$(status)"
T2=$(echo "$body" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# 1.7 Login с верным паролем.
body=$(req POST /api/auth/login "" "{\"username\":\"$U1\",\"password\":\"secret123\"}")
assert_eq "login U1 -> 200" 200 "$(status)"

# 1.8 Login с неверным паролем.
body=$(req POST /api/auth/login "" "{\"username\":\"$U1\",\"password\":\"wrong\"}")
assert_eq "login bad password -> 401" 401 "$(status)"

# 1.9 /me без токена — 401.
body=$(req GET /api/auth/me)
assert_eq "/me without token -> 401" 401 "$(status)"

# 1.10 /me с токеном.
body=$(req GET /api/auth/me "$T1")
assert_eq "/me with token -> 200" 200 "$(status)"
assert_in "/me returns username" "$U1" "$body"

# -------------------- 2. Управление пациентами (ТЗ 4.1.1 п.2) --------------------

section "2. Пациенты"

# 2.1 Создание пациента.
body=$(req POST /api/patients "$T1" "{\"first_name\":\"Иван\",\"last_name\":\"Иванов\",\"date_of_birth\":\"1990-05-15\",\"sex\":\"male\",\"email\":\"ivanov@ex.com\",\"external_id\":\"EXT-$TS-001\"}")
assert_eq "create patient -> 201" 201 "$(status)"
PID1=$(echo "$body" | python3 -c 'import sys,json;print(json.load(sys.stdin)["patient_id"])')
echo "  patient_id=$PID1 body=$(echo "$body" | head -c 200)"

# 2.2 Список.
body=$(req GET "/api/patients?limit=20" "$T1")
assert_eq "list patients -> 200" 200 "$(status)"
assert_in "list contains created patient" "EXT-$TS-001" "$body"

# 2.3 Get.
body=$(req GET "/api/patients/$PID1" "$T1")
assert_eq "get patient -> 200" 200 "$(status)"
assert_in "get returns last_name" "Иванов" "$body"

# 2.4 Изоляция: U2 не должен видеть пациента U1.
body=$(req GET "/api/patients/$PID1" "$T2")
assert_in "isolation: U2 sees U1's patient as 404/403" "$(status)" "404 403"
echo "  (status was $(status))"

# 2.5 Patch.
body=$(req PATCH "/api/patients/$PID1" "$T1" "{\"phenotype_description\":\"Гипертония\"}")
assert_eq "patch patient -> 200" 200 "$(status)"
assert_in "patch updated phenotype" "Гипертония" "$body"

# 2.6 Второй пациент для теста удаления и пагинации.
body=$(req POST /api/patients "$T1" "{\"first_name\":\"Пётр\",\"last_name\":\"Петров\",\"date_of_birth\":\"1985-01-01\",\"sex\":\"male\",\"external_id\":\"EXT-$TS-002\"}")
PID2=$(echo "$body" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("patient_id",""))')

# 2.7 Создание пациента у U2 (для проверки изоляции в списке).
body=$(req POST /api/patients "$T2" "{\"first_name\":\"Сидор\",\"last_name\":\"Сидоров\",\"date_of_birth\":\"1988-08-08\",\"sex\":\"male\",\"external_id\":\"EXT-$TS-U2-001\"}")
PID_U2=$(echo "$body" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("patient_id",""))')
body=$(req GET "/api/patients?limit=100" "$T1")
if echo "$body" | grep -q "EXT-$TS-U2-001"; then
    echo "  [FAIL] list isolation: U1 sees U2 patient"; FAIL=$((FAIL+1)); FAILED_TESTS+=("list-isolation")
else
    echo "  [OK]  list isolation: U1 does not see U2 patient"; PASS=$((PASS+1))
fi

# -------------------- 3. Ручное добавление варианта (ТЗ 4.1.1 п.3.1) --------------------

section "3. Ручное добавление варианта"

body=$(req POST "/api/patients/$PID1/variants" "$T1" "{\"chromosome\":\"1\",\"position\":985900,\"reference\":\"A\",\"alternate\":\"G\",\"rs_id\":\"rs12345\",\"variant_type\":\"SNV\",\"genome_build\":\"GRCh38\"}")
assert_eq "add manual variant -> 201" 201 "$(status)"
VID=$(echo "$body" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d.get("variant_id") or d.get("id") or "")')
echo "  variant_id=$VID, body=$(echo "$body" | head -c 200)"

# 3.2 Невалидный хромосом — 422.
body=$(req POST "/api/patients/$PID1/variants" "$T1" "{\"chromosome\":\"zz\",\"position\":100,\"reference\":\"A\",\"alternate\":\"G\",\"variant_type\":\"SNV\"}")
assert_eq "manual variant bad chrom -> 422" 422 "$(status)"

# 3.3 Невалидный аллель — 422.
body=$(req POST "/api/patients/$PID1/variants" "$T1" "{\"chromosome\":\"1\",\"position\":100,\"reference\":\"X\",\"alternate\":\"G\",\"variant_type\":\"SNV\"}")
assert_eq "manual variant bad ref -> 422" 422 "$(status)"

# 3.4 Дубль — 409.
body=$(req POST "/api/patients/$PID1/variants" "$T1" "{\"chromosome\":\"1\",\"position\":985900,\"reference\":\"A\",\"alternate\":\"G\",\"rs_id\":\"rs12345\",\"variant_type\":\"SNV\",\"genome_build\":\"GRCh38\"}")
assert_eq "manual variant duplicate -> 409" 409 "$(status)"

# 3.5 Список вариантов пациента.
body=$(req GET "/api/patients/$PID1/variants?limit=50" "$T1")
assert_eq "list patient variants -> 200" 200 "$(status)"
assert_in "patient variants contain rs12345" "rs12345" "$body"

# -------------------- 4. Импорт VCF и CSV (ТЗ 4.1.1 п.4) --------------------

section "4. Импорт VCF и CSV"

# 4.1 Маленький VCF.
cat > "$TMP/test.vcf" <<'EOF'
##fileformat=VCFv4.2
##INFO=<ID=DP,Number=1,Type=Integer,Description="Depth">
#CHROM	POS	ID	REF	ALT	QUAL	FILTER	INFO
1	1000100	rs9990001	C	T	100	PASS	DP=30
2	2000200	.	G	A	80	PASS	DP=25
17	41245466	rs80357906	G	A	200	PASS	DP=40
EOF
body=$(upload_file "/api/patients/$PID1/variants/vcf" "$T1" "file=$TMP/test.vcf")
assert_in "VCF import status" "$(status)" "200 201 202"
echo "  status=$(status) body=$(echo "$body" | head -c 200)"

# 4.2 CSV.
cat > "$TMP/test.csv" <<'EOF'
chromosome,position,reference_allele,alternate_allele,rs_id,genome_build,variant_type
3,3000300,A,T,rs7770001,GRCh38,SNV
4,4000400,GC,G,,GRCh38,DEL
5,5000500,T,TA,rs7770002,GRCh38,INS
EOF
body=$(upload_file "/api/patients/$PID1/variants/csv" "$T1" "file=$TMP/test.csv")
assert_in "CSV import status" "$(status)" "200 201 202"
echo "  status=$(status) body=$(echo "$body" | head -c 200)"

# 4.3 Список вариантов должен вырасти.
body=$(req GET "/api/patients/$PID1/variants?limit=200" "$T1")
N=$(echo "$body" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d.get("total") or len(d.get("items") or d.get("variants") or []))')
echo "  variants after import: $N"
if [ "${N:-0}" -ge 4 ]; then echo "  [OK]  imports increased count"; PASS=$((PASS+1)); else echo "  [FAIL] imports did not increase count"; FAIL=$((FAIL+1)); FAILED_TESTS+=("imports-count"); fi

# -------------------- 5. Поиск похожих мутаций (ТЗ 4.1.1 п.7) --------------------

section "5. Поиск похожих мутаций"

body=$(req GET "/api/variants?rs_id=rs12345" "$T1")
assert_eq "search by rs_id -> 200" 200 "$(status)"
assert_in "rs_id search result" "rs12345" "$body"

body=$(req GET "/api/variants?chrom=1&pos=985900&ref=A&alt=G&build=GRCh38" "$T1")
assert_eq "search by locus -> 200" 200 "$(status)"
assert_in "locus search result" "985900" "$body"

body=$(req GET "/api/variants?q=rs12345" "$T1")
assert_eq "global ?q= -> 200" 200 "$(status)"
assert_in "q= search result" "rs12345" "$body"

# Поиск по геному (gene) — пустой, но не должен падать.
body=$(req GET "/api/variants?gene=BRCA1" "$T1")
assert_eq "search by gene -> 200" 200 "$(status)"

# -------------------- 6. Экспорт (ТЗ 4.1.1 п.8) --------------------

section "6. Экспорт"

body=$(req GET "/api/patients/export?format=csv" "$T1")
assert_eq "export patients CSV -> 200" 200 "$(status)"
assert_in "patients CSV has header" "external_id" "$body"

body=$(req GET "/api/patients/export?format=json" "$T1")
assert_eq "export patients JSON -> 200" 200 "$(status)"
assert_in "patients JSON contains EXT" "EXT-$TS" "$body"

body=$(req GET "/api/patients/$PID1/variants/export?format=csv" "$T1")
assert_eq "export variants CSV -> 200" 200 "$(status)"
assert_in "variants CSV has chrom field" "chrom" "$body"

body=$(req GET "/api/patients/$PID1/variants/export?format=json" "$T1")
assert_eq "export variants JSON -> 200" 200 "$(status)"

# -------------------- 7. Деталка варианта + enrichment_status (ТЗ 4.1.1 п.6.4) --------------------

section "7. Деталка варианта (enrichment_status)"

# Возьмём id первого варианта пациента.
body=$(req GET "/api/patients/$PID1/variants?limit=1" "$T1")
ANY_VID=$(echo "$body" | python3 -c '
import sys, json
d = json.load(sys.stdin)
items = d.get("items") if isinstance(d, dict) else d
v = items[0] if items else {}
# variant_id может быть на верхнем уровне (search) или внутри .variant (patient list)
vid = v.get("variant_id") or (v.get("variant") or {}).get("variant_id") or v.get("id") or ""
print(vid)
')
echo "  picked variant_id=$ANY_VID"
if [ -n "$ANY_VID" ]; then
    body=$(req GET "/api/variants/$ANY_VID" "$T1")
    assert_eq "get variant details -> 200" 200 "$(status)"
    assert_in "details contain enrichment_status" "enrichment_status" "$body"
    echo "  details snippet: $(echo "$body" | head -c 400)"
fi

# -------------------- 8. FASTQ загрузка (ТЗ 4.1.1 п.4.3, 5) --------------------

section "8. FASTQ + пайплайн"

# Берём готовый sample из репо.
SE_SAMPLE="scripts/reads/samples/03_reads_with_snp.fa"
if [ -f "$SE_SAMPLE" ]; then
    # Преобразуем .fa в простой .fastq (одна последовательность, q=I).
    python3 - "$SE_SAMPLE" "$TMP/se.fastq" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
with open(src) as f, open(dst, 'w') as o:
    i=0; name=None; seq=[]
    def flush(name, seq, idx):
        s=''.join(seq)
        if not s: return
        o.write(f"@{name or f'read{idx}'}\n{s}\n+\n{'I'*len(s)}\n")
    for line in f:
        line=line.strip()
        if line.startswith('>'):
            flush(name, seq, i); name=line[1:].split()[0] or f'read{i}'; seq=[]; i+=1
        elif line:
            seq.append(line)
    flush(name, seq, i)
PY
    body=$(upload_file "/api/patients/$PID1/samples" "$T1" "file=$TMP/se.fastq")
    assert_in "SE FASTQ upload status" "$(status)" "200 201 202"
    SID=$(echo "$body" | python3 -c 'import sys,json
try:
    d=json.load(sys.stdin); print(d.get("sample_id") or d.get("id") or "")
except: print("")')
    echo "  sample_id=$SID body=$(echo "$body" | head -c 200)"
    if [ -n "$SID" ]; then
        # Poll до 60 секунд.
        for i in $(seq 1 30); do
            sleep 2
            body=$(req GET "/api/samples/$SID" "$T1")
            ST=$(echo "$body" | python3 -c 'import sys,json
try:
    d=json.load(sys.stdin)
    print(d.get("processing_status") or d.get("status") or "")
except: print("")')
            echo "  sample $SID status=$ST (try $i)"
            case "$ST" in completed|failed) break;; esac
        done
        if [ "$ST" = "completed" ] || [ "$ST" = "failed" ]; then
            echo "  [OK]  sample reached terminal status: $ST"; PASS=$((PASS+1))
        else
            echo "  [FAIL] sample did not reach terminal status in time (last=$ST)"; FAIL=$((FAIL+1)); FAILED_TESTS+=("fastq-pipeline-timeout")
        fi
    fi

    # 8.2 Paired-end (ТЗ 4.1.1 п.4.3).
    cp "$TMP/se.fastq" "$TMP/pe_r1.fastq"
    cp "$TMP/se.fastq" "$TMP/pe_r2.fastq"
    body=$(upload_file "/api/patients/$PID1/samples" "$T1" "file_r1=$TMP/pe_r1.fastq" "file_r2=$TMP/pe_r2.fastq")
    assert_in "PE FASTQ upload status" "$(status)" "200 201 202"
    echo "  PE body=$(echo "$body" | head -c 200)"
else
    echo "  (skipped: no sample file)"
fi

# -------------------- ИТОГ --------------------

section "Итого"
echo "Прошло: $PASS"
echo "Упало:  $FAIL"
if [ $FAIL -gt 0 ]; then
    echo "Упавшие:"; for t in "${FAILED_TESTS[@]}"; do echo "  - $t"; done
fi
[ $FAIL -eq 0 ]
