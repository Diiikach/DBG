import { useEffect, useMemo, useRef, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Checkbox,
  Code,
  Group,
  NumberInput,
  LoadingOverlay,
  Pagination,
  Paper,
  Popover,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Menu,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useDebouncedValue } from "@mantine/hooks";
import {
  IconAdjustments,
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconEye,
  IconDownload,
  IconFileTypeCsv,
  IconJson,
  IconSearch,
} from "@tabler/icons-react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
import { listPatientVariants } from "../api/variants";
import { exportPatientVariants, type ExportFormat } from "../api/export";
import { extractErrorMessage } from "../api/client";
import type {
  PatientVariantRich,
  VariantListParams,
} from "../types/api";
import { formatNumber, formatDateTime } from "../utils/format";
import { VariantDetailDrawer } from "./VariantDetailDrawer";

// Большой размер страницы — таблица выдержит благодаря виртуализации,
// а пользователь может просмотреть тысячи строк без лишних кликов
// пагинации (требование ТЗ 4.1.5 п.4 «виртуальный скроллинг»).
const PAGE_SIZE = 500;
const ROW_HEIGHT = 36;
const VIRTUAL_HEIGHT = 560;

const VARIANT_TYPES = ["SNV", "INS", "DEL", "INDEL", "MNV", "CNV", "SV", "OTHER"];
const ZYGOSITIES = [
  { value: "", label: "Любая" },
  { value: "HETEROZYGOUS", label: "Гетерозигота" },
  { value: "HOMOZYGOUS", label: "Гомозигота" },
  { value: "HEMIZYGOUS", label: "Гемизигота" },
];

type SortKey =
  | "chrom"
  | "position"
  | "quality"
  | "read_depth"
  | "variant_type"
  | "filter_status";

interface SortState {
  key: SortKey;
  dir: "asc" | "desc";
}

// API ждёт строку вида "chrom,position,-quality"
function buildSortParam(sort: SortState | null): string | undefined {
  if (!sort) return undefined;
  const prefix = sort.dir === "desc" ? "-" : "";
  return `${prefix}${sort.key}`;
}

// Колонки, доступные в списке вариантов.
// Все значения берутся либо из верхнеуровневых полей PatientVariantRich
// (top_consequence/top_impact/gene_symbols), либо из первой (топовой)
// аннотации в row.annotations[0]. Тем самым колонки, которые показывает
// детальный дровер, доступны и в самой таблице по клику «Колонки».
const OPTIONAL_COLUMNS = [
  { key: "consequence", label: "Consequence" },
  { key: "impact",      label: "Impact"      },
  { key: "genes",       label: "Гены"        },
  { key: "hgvsc",       label: "HGVSc"       },
  { key: "hgvsp",       label: "HGVSp"       },
  { key: "gnomad_af",   label: "gnomAD AF"   },
  { key: "gnomad_af_popmax", label: "gnomAD popmax" },
  { key: "sift",        label: "SIFT"        },
  { key: "polyphen",    label: "PolyPhen"    },
  { key: "cadd_score",  label: "CADD"        },
  { key: "revel_score", label: "REVEL"       },
  { key: "clinvar_significance", label: "ClinVar sig." },
  { key: "clinvar_id",  label: "ClinVar ID"  },
] as const;
type OptionalColumnKey = (typeof OPTIONAL_COLUMNS)[number]["key"];

const DEFAULT_OPTIONAL_COLS: OptionalColumnKey[] = [
  "consequence",
  "impact",
  "genes",
];

// Ключ, под которым набор видимых колонок хранится в localStorage
// (требование ТЗ 4.1.5 п.4: видимость столбцов сохраняется между сессиями).
const COLUMNS_STORAGE_KEY = "variantsTab.visibleCols.v1";

function loadVisibleCols(): Set<OptionalColumnKey> {
  try {
    const raw = localStorage.getItem(COLUMNS_STORAGE_KEY);
    if (!raw) return new Set(DEFAULT_OPTIONAL_COLS);
    const arr = JSON.parse(raw) as unknown;
    if (!Array.isArray(arr)) return new Set(DEFAULT_OPTIONAL_COLS);
    const allowed = new Set(OPTIONAL_COLUMNS.map((c) => c.key));
    return new Set(
      arr.filter((x): x is OptionalColumnKey =>
        typeof x === "string" && allowed.has(x as OptionalColumnKey),
      ),
    );
  } catch {
    return new Set(DEFAULT_OPTIONAL_COLS);
  }
}

function saveVisibleCols(set: Set<OptionalColumnKey>): void {
  try {
    localStorage.setItem(COLUMNS_STORAGE_KEY, JSON.stringify([...set]));
  } catch {
    /* localStorage недоступен — молча игнорируем */
  }
}

interface VariantsTabProps {
  patientId: number;
}

export function VariantsTab({ patientId }: VariantsTabProps) {
  const [chrom, setChrom] = useState("");
  const [variantType, setVariantType] = useState<string | null>(null);
  const [filterEq, setFilterEq] = useState("");
  const [minQual, setMinQual] = useState<number | string>("");
  const [zygosity, setZygosity] = useState<string | null>(null);
  const [sort, setSort] = useState<SortState | null>({ key: "chrom", dir: "asc" });
  const [page, setPage] = useState(1);
  const [visibleCols, setVisibleCols] = useState<Set<OptionalColumnKey>>(
    () => loadVisibleCols(),
  );
  const [globalSearch, setGlobalSearch] = useState("");
  // Дебаунс на 300 мс, чтобы не дёргать сервер на каждый символ.
  const [debouncedSearch] = useDebouncedValue(globalSearch.trim(), 300);
  const [openVariantId, setOpenVariantId] = useState<number | null>(null);
  const [exporting, setExporting] = useState(false);
  const tableTopRef = useRef<HTMLDivElement | null>(null);
  const scrollParentRef = useRef<HTMLDivElement | null>(null);

  // Сохраняем выбор колонок в localStorage при каждом изменении.
  useEffect(() => {
    saveVisibleCols(visibleCols);
  }, [visibleCols]);

  const params: VariantListParams = useMemo(
    () => ({
      chrom: chrom.trim() || undefined,
      variant_type: variantType || undefined,
      filter: filterEq.trim() || undefined,
      min_qual: typeof minQual === "number" ? minQual : undefined,
      zygosity: zygosity || undefined,
      q: debouncedSearch || undefined,
      sort: buildSortParam(sort),
      limit: PAGE_SIZE,
      offset: (page - 1) * PAGE_SIZE,
    }),
    [chrom, variantType, filterEq, minQual, zygosity, debouncedSearch, sort, page],
  );

  const { data, isLoading, isFetching, isError } = useQuery({
    queryKey: ["variants", patientId, params],
    queryFn: () => listPatientVariants(patientId, params),
    placeholderData: keepPreviousData,
  });

  // При смене страницы прокручиваем к началу таблицы — иначе пользователь,
  // кликнувший по пагинации внизу длинной таблицы, не видит изменений.
  useEffect(() => {
    tableTopRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [page]);

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Поиск выполняется на сервере (параметр q), поэтому здесь просто
  // используем уже отфильтрованный список с бэкенда.
  const filteredItems: PatientVariantRich[] = data?.items ?? [];

  // Если пользователь меняет поисковый запрос, сбрасываем пагинацию,
  // чтобы offset не указывал на несуществующую страницу нового результата.
  useEffect(() => {
    setPage(1);
  }, [debouncedSearch]);

  function toggleSort(key: SortKey) {
    setSort((s) => {
      if (!s || s.key !== key) return { key, dir: "asc" };
      if (s.dir === "asc") return { key, dir: "desc" };
      return null;
    });
    setPage(1);
  }

  function toggleCol(k: OptionalColumnKey, checked: boolean) {
    setVisibleCols((prev) => {
      const next = new Set(prev);
      if (checked) next.add(k);
      else next.delete(k);
      return next;
    });
  }

  function resetFilters() {
    setChrom("");
    setVariantType(null);
    setFilterEq("");
    setMinQual("");
    setZygosity(null);
    setGlobalSearch("");
    setPage(1);
  }

  const visibleOptCols = OPTIONAL_COLUMNS.filter((c) => visibleCols.has(c.key));
  const colSpan = 14 + visibleOptCols.length;

  async function handleExport(format: ExportFormat) {
    setExporting(true);
    try {
      // Передаём те же фильтры/сортировку, что и в таблице — limit/offset
      // отбрасываются на стороне клиента, бэкенд сам ограничивает выгрузку.
      await exportPatientVariants(patientId, format, params);
    } catch (err) {
      notifications.show({
        color: "red",
        title: "Не удалось выгрузить варианты",
        message: extractErrorMessage(err),
      });
    } finally {
      setExporting(false);
    }
  }

  // Виртуализация строк: рендерим только те строки, которые
  // действительно видны во вьюпорте контейнера прокрутки.
  const rowVirtualizer = useVirtualizer({
    count: filteredItems.length,
    getScrollElement: () => scrollParentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });

  const virtualItems = rowVirtualizer.getVirtualItems();
  const totalSize = rowVirtualizer.getTotalSize();
  const paddingTop = virtualItems[0]?.start ?? 0;
  const paddingBottom = totalSize - (virtualItems[virtualItems.length - 1]?.end ?? 0);

  return (
    <Stack>
      <Paper p="md" withBorder>
        <Group justify="space-between" mb="sm">
          <Title order={4}>Фильтры</Title>
          <Group>
            <Popover position="bottom-end" withArrow>
              <Popover.Target>
                <Button
                  variant="default"
                  leftSection={<IconAdjustments size={16} />}
                >
                  Колонки
                </Button>
              </Popover.Target>
              <Popover.Dropdown>
                <Stack gap={4}>
                  {OPTIONAL_COLUMNS.map((c) => (
                    <Checkbox
                      key={c.key}
                      label={c.label}
                      checked={visibleCols.has(c.key)}
                      onChange={(e) => toggleCol(c.key, e.currentTarget.checked)}
                    />
                  ))}
                </Stack>
              </Popover.Dropdown>
            </Popover>
            <Button variant="subtle" onClick={resetFilters}>
              Сбросить
            </Button>
          </Group>
        </Group>
        <Group grow>
          <TextInput
            label="Хромосома"
            placeholder="например chr1"
            value={chrom}
            onChange={(e) => {
              setChrom(e.currentTarget.value);
              setPage(1);
            }}
          />
          <Select
            label="Тип"
            data={VARIANT_TYPES}
            clearable
            value={variantType}
            onChange={(v) => {
              setVariantType(v);
              setPage(1);
            }}
          />
          <TextInput
            label="Filter"
            placeholder="например PASS"
            value={filterEq}
            onChange={(e) => {
              setFilterEq(e.currentTarget.value);
              setPage(1);
            }}
          />
          <NumberInput
            label="Min QUAL"
            value={minQual}
            onChange={(v) => {
              setMinQual(v);
              setPage(1);
            }}
            min={0}
            decimalScale={2}
          />
          <Select
            label="Зиготность"
            data={ZYGOSITIES.map((z) => ({ value: z.value, label: z.label }))}
            value={zygosity ?? ""}
            onChange={(v) => {
              setZygosity(v || null);
              setPage(1);
            }}
          />
        </Group>
      </Paper>

      <Paper p="md" withBorder pos="relative">
        <div ref={tableTopRef} />
        <LoadingOverlay
          visible={isFetching && !isLoading}
          zIndex={1}
          overlayProps={{ blur: 1, backgroundOpacity: 0.4 }}
        />
        <Group justify="space-between" mb="sm" wrap="wrap">
          <Group>
            <Title order={4}>Варианты</Title>
            <TextInput
              placeholder="Поиск (rsID, ген, chr1:12345, HGVS, ClinVar…)"
              leftSection={<IconSearch size={14} />}
              value={globalSearch}
              onChange={(e) => setGlobalSearch(e.currentTarget.value)}
              w={360}
              size="xs"
            />
          </Group>
          <Group gap="xs">
            {isFetching && !isLoading && (
              <Text size="sm" c="dimmed">Обновление…</Text>
            )}
            <Text size="sm" c="dimmed">
              {debouncedSearch
                ? `Найдено по запросу: ${total}`
                : `Найдено: ${total}`}
            </Text>
            <Menu shadow="md" position="bottom-end">
              <Menu.Target>
                <Button
                  variant="default"
                  size="xs"
                  leftSection={<IconDownload size={14} />}
                  loading={exporting}
                  disabled={total === 0}
                >
                  Скачать
                </Button>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Label>Экспорт вариантов (с учётом фильтров)</Menu.Label>
                <Menu.Item
                  leftSection={<IconFileTypeCsv size={16} />}
                  onClick={() => handleExport("csv")}
                >
                  CSV
                </Menu.Item>
                <Menu.Item
                  leftSection={<IconJson size={16} />}
                  onClick={() => handleExport("json")}
                >
                  JSON
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        </Group>

        {isError ? (
          <Alert color="red" icon={<IconAlertCircle size={16} />}>
            Не удалось загрузить варианты
          </Alert>
        ) : isLoading ? (
          <Stack>
            {Array.from({ length: 8 }).map((_, i) => (
              <div
                key={i}
                style={{
                  height: ROW_HEIGHT,
                  borderRadius: 4,
                  background:
                    "linear-gradient(90deg, var(--mantine-color-gray-1) 0%, var(--mantine-color-gray-2) 50%, var(--mantine-color-gray-1) 100%)",
                  backgroundSize: "200% 100%",
                  animation: "vt-shimmer 1.4s ease-in-out infinite",
                }}
              />
            ))}
            <style>{`
              @keyframes vt-shimmer {
                0%   { background-position: 200% 0; }
                100% { background-position: -200% 0; }
              }
            `}</style>
          </Stack>
        ) : (
          <div
            ref={scrollParentRef}
            style={{
              maxHeight: VIRTUAL_HEIGHT,
              overflow: "auto",
              border: "1px solid var(--mantine-color-gray-3)",
              borderRadius: 4,
            }}
          >
            <Table striped highlightOnHover stickyHeader>
              <Table.Thead>
                <Table.Tr>
                  <SortableTh label="Chr"    k="chrom"         sort={sort} onToggle={toggleSort} />
                  <SortableTh label="Pos"    k="position"      sort={sort} onToggle={toggleSort} />
                  <Table.Th>Ref/Alt</Table.Th>
                  <SortableTh label="Тип"    k="variant_type"  sort={sort} onToggle={toggleSort} />
                  <Table.Th>rsID</Table.Th>
                  <Table.Th>Build</Table.Th>
                  <Table.Th>Зиготность</Table.Th>
                  <SortableTh label="QUAL"   k="quality"       sort={sort} onToggle={toggleSort} />
                  <SortableTh label="DP"     k="read_depth"    sort={sort} onToggle={toggleSort} />
                  <Table.Th>AD ref/alt</Table.Th>
                  <Table.Th>GQ</Table.Th>
                  <SortableTh label="Filter" k="filter_status" sort={sort} onToggle={toggleSort} />
                  <Table.Th>Detected</Table.Th>
                  {visibleOptCols.map((c) => (
                    <Table.Th key={c.key}>{c.label}</Table.Th>
                  ))}
                  <Table.Th style={{ width: 60 }}></Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {filteredItems.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={colSpan + 1}>
                      <Text c="dimmed" ta="center" py="md">
                        Варианты не найдены
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  <>
                    {paddingTop > 0 && (
                      <Table.Tr style={{ height: paddingTop }}>
                        <Table.Td colSpan={colSpan + 1} style={{ padding: 0, border: "none" }} />
                      </Table.Tr>
                    )}
                    {virtualItems.map((vi) => {
                      const row = filteredItems[vi.index];
                      return (
                        <VariantRow
                          key={row.patient_variant_id}
                          row={row}
                          visibleCols={visibleCols}
                          onOpen={() => setOpenVariantId(row.variant.variant_id)}
                        />
                      );
                    })}
                    {paddingBottom > 0 && (
                      <Table.Tr style={{ height: paddingBottom }}>
                        <Table.Td colSpan={colSpan + 1} style={{ padding: 0, border: "none" }} />
                      </Table.Tr>
                    )}
                  </>
                )}
              </Table.Tbody>
            </Table>
          </div>
        )}

        {totalPages > 1 && (
          <Group justify="center" mt="md">
            <Pagination total={totalPages} value={page} onChange={setPage} />
          </Group>
        )}
      </Paper>

      <VariantDetailDrawer
        variantId={openVariantId}
        opened={openVariantId !== null}
        onClose={() => setOpenVariantId(null)}
      />
    </Stack>
  );
}

function SortableTh({
  label,
  k,
  sort,
  onToggle,
}: {
  label: string;
  k: SortKey;
  sort: SortState | null;
  onToggle: (k: SortKey) => void;
}) {
  const active = sort?.key === k;
  return (
    <Table.Th
      style={{ cursor: "pointer", whiteSpace: "nowrap" }}
      onClick={() => onToggle(k)}
    >
      <Group gap={4} wrap="nowrap">
        {label}
        {active ? (
          sort!.dir === "asc" ? (
            <IconArrowUp size={12} />
          ) : (
            <IconArrowDown size={12} />
          )
        ) : (
          <IconArrowsSort size={12} opacity={0.4} />
        )}
      </Group>
    </Table.Th>
  );
}

function VariantRow({
  row,
  visibleCols,
  onOpen,
}: {
  row: PatientVariantRich;
  visibleCols: Set<OptionalColumnKey>;
  onOpen: () => void;
}) {
  const v = row.variant;

  return (
    <Table.Tr style={{ cursor: "pointer" }} onClick={onOpen}>
      <Table.Td>{v.chromosome}</Table.Td>
      <Table.Td>{v.position}</Table.Td>
      <Table.Td>
        <RefAltCell ref_={v.reference} alt={v.alternate} />
      </Table.Td>
      <Table.Td>{v.variant_type ?? "—"}</Table.Td>
      <Table.Td>{v.rs_id ?? "—"}</Table.Td>
      <Table.Td>{v.genome_build ?? "—"}</Table.Td>
      <Table.Td>{row.zygosity ?? "—"}</Table.Td>
      <Table.Td>{formatNumber(row.quality)}</Table.Td>
      <Table.Td>{row.read_depth ?? "—"}</Table.Td>
      <Table.Td>
        {row.allele_depth_ref ?? "—"}/{row.allele_depth_alt ?? "—"}
      </Table.Td>
      <Table.Td>{row.genotype_quality ?? "—"}</Table.Td>
      <Table.Td>
        {row.filter_status ? (
          <Badge
            color={row.filter_status === "PASS" ? "green" : "yellow"}
            variant="light"
          >
            {row.filter_status}
          </Badge>
        ) : (
          "—"
        )}
      </Table.Td>
      <Table.Td>{formatDateTime(row.detected_at)}</Table.Td>

      {/* Опциональные колонки — данные берём из полей PatientVariantRich */}
      {OPTIONAL_COLUMNS.filter((c) => visibleCols.has(c.key)).map((c) => (
        <Table.Td key={c.key}>
          {renderRichCell(c.key, row)}
        </Table.Td>
      ))}

      <Table.Td onClick={(e) => e.stopPropagation()}>
        <ActionIcon variant="subtle" onClick={onOpen} aria-label="Открыть">
          <IconEye size={16} />
        </ActionIcon>
      </Table.Td>
    </Table.Tr>
  );
}

const MAX_ALLELE_LEN = 10;

function RefAltCell({ ref_, alt }: { ref_: string; alt: string }) {
  const refShort = ref_.length > MAX_ALLELE_LEN ? ref_.slice(0, MAX_ALLELE_LEN) + "…" : ref_;
  const altShort = alt.length > MAX_ALLELE_LEN ? alt.slice(0, MAX_ALLELE_LEN) + "…" : alt;
  const needsPopover = ref_.length > MAX_ALLELE_LEN || alt.length > MAX_ALLELE_LEN;

  const label = (
    <Text size="sm" ff="monospace" style={{ whiteSpace: "nowrap" }}>
      {refShort} → {altShort}
    </Text>
  );

  if (!needsPopover) return label;

  return (
    <Popover width={320} position="right" withArrow shadow="md">
      <Popover.Target>
        <Text
          size="sm"
          ff="monospace"
          style={{ whiteSpace: "nowrap", cursor: "pointer", textDecoration: "underline dotted" }}
          onClick={(e) => e.stopPropagation()}
        >
          {refShort} → {altShort}
        </Text>
      </Popover.Target>
      <Popover.Dropdown onClick={(e) => e.stopPropagation()}>
        <Text size="xs" c="dimmed" mb={4}>Ref</Text>
        <Code block style={{ wordBreak: "break-all", fontSize: 11 }}>{ref_}</Code>
        <Text size="xs" c="dimmed" mt={8} mb={4}>Alt</Text>
        <Code block style={{ wordBreak: "break-all", fontSize: 11 }}>{alt}</Code>
      </Popover.Dropdown>
    </Popover>
  );
}

/**
 * Рендерит ячейку опциональной колонки.
 * top_consequence / top_impact / gene_symbols берём из верхнеуровневых
 * полей PatientVariantRich. Остальные поля — из row.annotations[0]
 * (топовая аннотация, та же, что показывается первой строкой в дровере).
 */
function renderRichCell(key: OptionalColumnKey, row: PatientVariantRich): React.ReactNode {
  const ann = row.annotations?.[0];

  switch (key) {
    case "consequence": {
      const v = row.top_consequence ?? ann?.consequence ?? null;
      return v ? <Text size="xs" ff="monospace">{v}</Text> : "—";
    }

    case "impact": {
      const v = row.top_impact ?? ann?.impact ?? null;
      if (!v) return "—";
      return (
        <Badge
          size="xs"
          variant="light"
          color={
            v === "HIGH"     ? "red"    :
            v === "MODERATE" ? "orange" :
            v === "LOW"      ? "yellow" : "gray"
          }
        >
          {v}
        </Badge>
      );
    }

    case "genes":
      return (row.gene_symbols ?? []).join(", ") || "—";

    case "hgvsc":
      return ann?.hgvsc ? <Text size="xs" ff="monospace">{ann.hgvsc}</Text> : "—";

    case "hgvsp":
      return ann?.hgvsp ? <Text size="xs" ff="monospace">{ann.hgvsp}</Text> : "—";

    case "gnomad_af":
      return formatNumber(ann?.gnomad_af, 5);

    case "gnomad_af_popmax":
      return formatNumber(ann?.gnomad_af_popmax, 5);

    case "sift": {
      const s = formatNumber(ann?.sift_score, 3);
      return ann?.sift_prediction ? `${s} (${ann.sift_prediction})` : s;
    }

    case "polyphen": {
      const s = formatNumber(ann?.polyphen_score, 3);
      return ann?.polyphen_prediction ? `${s} (${ann.polyphen_prediction})` : s;
    }

    case "cadd_score":
      return formatNumber(ann?.cadd_score, 2);

    case "revel_score":
      return formatNumber(ann?.revel_score, 3);

    case "clinvar_significance":
      return ann?.clinvar_significance ?? "—";

    case "clinvar_id":
      return ann?.clinvar_id ?? "—";
  }
}
