import { useState } from "react";
import {
  ActionIcon,
  Anchor,
  Button,
  Group,
  Menu,
  Pagination,
  Paper,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Skeleton,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useDebouncedValue } from "@mantine/hooks";
import {
  IconPlus,
  IconSearch,
  IconEye,
  IconDownload,
  IconFileTypeCsv,
  IconJson,
} from "@tabler/icons-react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listPatients } from "../api/patients";
import { exportPatients, type ExportFormat } from "../api/export";
import { extractErrorMessage } from "../api/client";
import { formatDate } from "../utils/format";

const PAGE_SIZE = 25;

export function PatientsListPage() {
  const [q, setQ] = useState("");
  const [debouncedQ] = useDebouncedValue(q.trim(), 300);
  const [page, setPage] = useState(1);
  const [exporting, setExporting] = useState(false);

  async function handleExport(format: ExportFormat) {
    setExporting(true);
    try {
      await exportPatients(format, { q: debouncedQ || undefined });
    } catch (err) {
      notifications.show({
        color: "red",
        title: "Не удалось выгрузить пациентов",
        message: extractErrorMessage(err),
      });
    } finally {
      setExporting(false);
    }
  }

  const { data, isLoading, isError } = useQuery({
    queryKey: ["patients", { q: debouncedQ, page }],
    queryFn: () =>
      listPatients({
        q: debouncedQ || undefined,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      }),
    placeholderData: keepPreviousData,
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={2}>Пациенты</Title>
        <Group>
          <Menu shadow="md" position="bottom-end">
            <Menu.Target>
              <Button
                variant="default"
                leftSection={<IconDownload size={16} />}
                loading={exporting}
              >
                Скачать
              </Button>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Label>Экспорт списка пациентов</Menu.Label>
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
          <Button
            component={Link}
            to="/patients/new"
            leftSection={<IconPlus size={16} />}
          >
            Добавить пациента
          </Button>
        </Group>
      </Group>

      <Paper p="md" withBorder>
        <Group mb="md">
          <TextInput
            placeholder="Поиск по ФИО / external_id / email"
            leftSection={<IconSearch size={16} />}
            value={q}
            onChange={(e) => {
              setQ(e.currentTarget.value);
              setPage(1);
            }}
            style={{ flex: 1, maxWidth: 480 }}
          />
          <Text size="sm" c="dimmed">
            Всего: {total}
          </Text>
        </Group>

        {isError ? (
          <Text c="red">Не удалось загрузить список пациентов</Text>
        ) : isLoading ? (
          <Stack>
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} h={32} />
            ))}
          </Stack>
        ) : (
          <>
            <Table striped highlightOnHover withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>External ID</Table.Th>
                  <Table.Th>ФИО</Table.Th>
                  <Table.Th>Дата рождения</Table.Th>
                  <Table.Th>Пол</Table.Th>
                  <Table.Th>Email</Table.Th>
                  <Table.Th style={{ width: 60 }}></Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {(data?.items ?? []).map((p) => (
                  <Table.Tr key={p.patient_id}>
                    <Table.Td>{p.external_id ?? "—"}</Table.Td>
                    <Table.Td>
                      <Anchor component={Link} to={`/patients/${p.patient_id}`}>
                        {p.last_name} {p.first_name}
                      </Anchor>
                    </Table.Td>
                    <Table.Td>{formatDate(p.date_of_birth)}</Table.Td>
                    <Table.Td>{p.sex ?? "—"}</Table.Td>
                    <Table.Td>{p.email ?? "—"}</Table.Td>
                    <Table.Td>
                      <ActionIcon
                        component={Link}
                        to={`/patients/${p.patient_id}`}
                        variant="subtle"
                        aria-label="Открыть"
                      >
                        <IconEye size={16} />
                      </ActionIcon>
                    </Table.Td>
                  </Table.Tr>
                ))}
                {(data?.items?.length ?? 0) === 0 && (
                  <Table.Tr>
                    <Table.Td colSpan={6}>
                      <Text c="dimmed" ta="center" py="md">
                        Пациенты не найдены
                      </Text>
                    </Table.Td>
                  </Table.Tr>
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
