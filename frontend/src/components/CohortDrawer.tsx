import {
  Anchor,
  Drawer,
  Group,
  Pagination,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useState } from "react";
import { getVariantCohort } from "../api/variants";
import { formatDate } from "../utils/format";

const PAGE_SIZE = 25;

export interface CohortDrawerProps {
  variantId: number | null;
  opened: boolean;
  onClose: () => void;
}

export function CohortDrawer({ variantId, opened, onClose }: CohortDrawerProps) {
  const [page, setPage] = useState(1);

  const { data, isLoading, isError } = useQuery({
    queryKey: ["cohort", variantId, { page }],
    queryFn: () =>
      getVariantCohort(variantId!, {
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      }),
    enabled: opened && variantId !== null,
    placeholderData: keepPreviousData,
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <Drawer
      opened={opened}
      onClose={onClose}
      title={<Title order={4}>Пациенты с этим вариантом</Title>}
      position="right"
      size="lg"
    >
      <Stack>
        <Text size="sm" c="dimmed">
          Всего пациентов: {total}
        </Text>
        {isError ? (
          <Text c="red">Не удалось загрузить когорту</Text>
        ) : isLoading ? (
          <Text c="dimmed">Загрузка…</Text>
        ) : (
          <>
            <Table striped withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>External ID</Table.Th>
                  <Table.Th>ФИО</Table.Th>
                  <Table.Th>Дата рожд.</Table.Th>
                  <Table.Th>Пол</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {(data?.items ?? []).map((p) => (
                  <Table.Tr key={p.patient_id}>
                    <Table.Td>{p.external_id ?? "—"}</Table.Td>
                    <Table.Td>
                      <Anchor
                        component={Link}
                        to={`/patients/${p.patient_id}`}
                        onClick={onClose}
                      >
                        {p.last_name} {p.first_name}
                      </Anchor>
                    </Table.Td>
                    <Table.Td>{formatDate(p.date_of_birth)}</Table.Td>
                    <Table.Td>{p.sex ?? "—"}</Table.Td>
                  </Table.Tr>
                ))}
                {(data?.items?.length ?? 0) === 0 && (
                  <Table.Tr>
                    <Table.Td colSpan={4}>
                      <Text c="dimmed" ta="center" py="md">
                        Никого не найдено
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                )}
              </Table.Tbody>
            </Table>
            {totalPages > 1 && (
              <Group justify="center">
                <Pagination total={totalPages} value={page} onChange={setPage} />
              </Group>
            )}
          </>
        )}
      </Stack>
    </Drawer>
  );
}
