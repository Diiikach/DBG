import { useState } from "react";
import {
  Alert,
  Anchor,
  Group,
  Pagination,
  Paper,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { IconAlertCircle } from "@tabler/icons-react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listAllSamples } from "../api/samples";
import { SampleStatusBadge } from "../components/SampleStatusBadge";
import { formatDateTime } from "../utils/format";

const PAGE_SIZE = 50;

/**
 * Глобальный список загрузок — все sample во всей доступной пользователю БД
 * (GET /api/samples), без выбора пациента.
 */
export function SamplesListPage() {
  const [page, setPage] = useState(1);

  const { data, isLoading, isError } = useQuery({
    queryKey: ["samples", "global", page],
    queryFn: () =>
      listAllSamples({ limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }),
    placeholderData: keepPreviousData,
    // Лёгкий авто-рефреш — если есть processing, статусы обновятся.
    refetchInterval: 5000,
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const items = data?.items ?? [];

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={2}>Все загрузки</Title>
        <Text size="sm" c="dimmed">
          Всего: {total}
        </Text>
      </Group>

      <Paper p="md" withBorder>
        {isError ? (
          <Alert color="red" icon={<IconAlertCircle size={16} />}>
            Не удалось загрузить список образцов
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
                  <Table.Th>ID</Table.Th>
                  <Table.Th>Название</Table.Th>
                  <Table.Th>Пациент</Table.Th>
                  <Table.Th>Тип</Table.Th>
                  <Table.Th>Секвенирование</Table.Th>
                  <Table.Th>Создан</Table.Th>
                  <Table.Th>Статус</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {items.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={7}>
                      <Text c="dimmed" ta="center" py="md">
                        Загрузок пока нет
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  items.map((s) => (
                    <Table.Tr key={s.sample_id}>
                      <Table.Td>{s.sample_id}</Table.Td>
                      <Table.Td>{s.sample_name}</Table.Td>
                      <Table.Td>
                        <Anchor
                          component={Link}
                          to={`/patients/${s.patient_id}`}
                        >
                          {s.patient_last_name} {s.patient_first_name}
                          {s.patient_external_id ? ` (${s.patient_external_id})` : ""}
                        </Anchor>
                      </Table.Td>
                      <Table.Td>{s.sample_type}</Table.Td>
                      <Table.Td>{s.sequencing_type}</Table.Td>
                      <Table.Td>{formatDateTime(s.created_at)}</Table.Td>
                      <Table.Td>
                        <SampleStatusBadge
                          status={s.processing_status}
                          failureReason={s.failure_reason}
                        />
                      </Table.Td>
                    </Table.Tr>
                  ))
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
    </Stack>
  );
}
