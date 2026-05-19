import { useEffect, useMemo, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Group,
  Pagination,
  Paper,
  Skeleton,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useDebouncedValue } from "@mantine/hooks";
import {
  IconAlertCircle,
  IconEye,
  IconSearch,
} from "@tabler/icons-react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { searchAllVariants } from "../api/variants";
import { VariantDetailDrawer } from "../components/VariantDetailDrawer";

const PAGE_SIZE = 50;

/**
 * Глобальный список вариантов — все варианты во всей БД (GET /api/variants),
 * без открытия через пациентов. Поиск по rsID/гену/координатам/глобальному q.
 */
export function VariantsListPage() {
  const [q, setQ] = useState("");
  const [debouncedQ] = useDebouncedValue(q.trim(), 300);
  const [page, setPage] = useState(1);
  const [openVariantId, setOpenVariantId] = useState<number | null>(null);

  useEffect(() => {
    setPage(1);
  }, [debouncedQ]);

  const params = useMemo(
    () => ({
      q: debouncedQ || undefined,
      limit: PAGE_SIZE,
      offset: (page - 1) * PAGE_SIZE,
    }),
    [debouncedQ, page],
  );

  const { data, isLoading, isError } = useQuery({
    queryKey: ["variants", "global", params],
    queryFn: () => searchAllVariants(params),
    placeholderData: keepPreviousData,
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const items = data?.items ?? [];

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={2}>Все варианты</Title>
      </Group>

      <Paper p="md" withBorder>
        <Group mb="md">
          <TextInput
            placeholder="Поиск (rsID, ген, chr1:12345…)"
            leftSection={<IconSearch size={16} />}
            value={q}
            onChange={(e) => setQ(e.currentTarget.value)}
            style={{ flex: 1, maxWidth: 480 }}
          />
          <Text size="sm" c="dimmed">
            Всего: {total}
          </Text>
        </Group>

        {isError ? (
          <Alert color="red" icon={<IconAlertCircle size={16} />}>
            Не удалось загрузить варианты
          </Alert>
        ) : isLoading ? (
          <Stack>
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} h={32} />
            ))}
          </Stack>
        ) : (
          <>
            <Table striped highlightOnHover withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Chr</Table.Th>
                  <Table.Th>Pos</Table.Th>
                  <Table.Th>Ref/Alt</Table.Th>
                  <Table.Th>Тип</Table.Th>
                  <Table.Th>rsID</Table.Th>
                  <Table.Th>Build</Table.Th>
                  <Table.Th>Гены</Table.Th>
                  <Table.Th>Impact</Table.Th>
                  <Table.Th>ClinVar</Table.Th>
                  <Table.Th>Пациентов</Table.Th>
                  <Table.Th style={{ width: 60 }}></Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {items.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={11}>
                      <Text c="dimmed" ta="center" py="md">
                        Варианты не найдены
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  items.map((r) => {
                    const v = r.variant;
                    return (
                      <Table.Tr
                        key={v.variant_id}
                        style={{ cursor: "pointer" }}
                        onClick={() => setOpenVariantId(v.variant_id)}
                      >
                        <Table.Td>{v.chromosome}</Table.Td>
                        <Table.Td>{v.position}</Table.Td>
                        <Table.Td>
                          <Text size="sm" ff="monospace">
                            {truncate(v.reference)} → {truncate(v.alternate)}
                          </Text>
                        </Table.Td>
                        <Table.Td>{v.variant_type ?? "—"}</Table.Td>
                        <Table.Td>{v.rs_id ?? "—"}</Table.Td>
                        <Table.Td>{v.genome_build ?? "—"}</Table.Td>
                        <Table.Td>
                          {(r.gene_symbols ?? []).join(", ") || "—"}
                        </Table.Td>
                        <Table.Td>
                          {r.top_impact ? (
                            <Badge
                              size="xs"
                              variant="light"
                              color={
                                r.top_impact === "HIGH"
                                  ? "red"
                                  : r.top_impact === "MODERATE"
                                    ? "orange"
                                    : r.top_impact === "LOW"
                                      ? "yellow"
                                      : "gray"
                              }
                            >
                              {r.top_impact}
                            </Badge>
                          ) : (
                            "—"
                          )}
                        </Table.Td>
                        <Table.Td>{r.clinvar_significance ?? "—"}</Table.Td>
                        <Table.Td>{r.patient_count}</Table.Td>
                        <Table.Td onClick={(e) => e.stopPropagation()}>
                          <ActionIcon
                            variant="subtle"
                            onClick={() => setOpenVariantId(v.variant_id)}
                            aria-label="Открыть"
                          >
                            <IconEye size={16} />
                          </ActionIcon>
                        </Table.Td>
                      </Table.Tr>
                    );
                  })
                )}
              </Table.Tbody>
            </Table>

            {totalPages > 1 && (
              <Group justify="center" mt="md">
                <Pagination total={totalPages} value={page} onChange={setPage} />
              </Group>
            )}
          </>
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

const MAX_ALLELE_LEN = 10;
function truncate(s: string): string {
  return s.length > MAX_ALLELE_LEN ? s.slice(0, MAX_ALLELE_LEN) + "…" : s;
}
