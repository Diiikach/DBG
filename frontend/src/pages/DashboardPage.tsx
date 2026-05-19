import {
  Anchor,
  Badge,
  Button,
  Card,
  Group,
  Paper,
  Skeleton,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { IconPlus, IconUsers, IconDna, IconFileUpload } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { listPatients } from "../api/patients";
import { listAllSamples } from "../api/samples";
import { searchAllVariants } from "../api/variants";
import { useAuth } from "../auth/useAuth";
import { formatDate } from "../utils/format";

const RECENT_LIMIT = 5;

/**
 * Дашборд (требование 4.1.5 ТЗ): сводка по пациентам, быстрые действия,
 * последние созданные карточки. Очередь задач и формы добавления вариантов
 * выведены ссылками-заглушками: соответствующих ручек у бэкенда пока нет.
 */
export function DashboardPage() {
  const { user } = useAuth();

  const totalQuery = useQuery({
    queryKey: ["patients", "total"],
    queryFn: () => listPatients({ limit: 1, offset: 0 }),
  });

  const recentQuery = useQuery({
    queryKey: ["patients", "recent"],
    queryFn: () => listPatients({ limit: RECENT_LIMIT, offset: 0 }),
  });

  const variantsTotalQuery = useQuery({
    queryKey: ["variants", "global", "total"],
    queryFn: () => searchAllVariants({ limit: 1, offset: 0 }),
  });

  const samplesTotalQuery = useQuery({
    queryKey: ["samples", "global", "total"],
    queryFn: () => listAllSamples({ limit: 1, offset: 0 }),
  });

  const total = totalQuery.data?.total ?? 0;
  const recent = recentQuery.data?.items ?? [];

  return (
    <Stack>
      <Group justify="space-between">
        <div>
          <Title order={2}>Дашборд</Title>
          {user && (
            <Text c="dimmed" size="sm">
              Добро пожаловать, {user.username}
            </Text>
          )}
        </div>
        <Group>
          <Button
            component={Link}
            to="/patients/new"
            leftSection={<IconPlus size={16} />}
          >
            Добавить пациента
          </Button>
        </Group>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
        <StatCard
          label="Пациентов"
          value={totalQuery.isLoading ? null : total}
          icon={<IconUsers size={20} />}
          to="/patients"
        />
        <StatCard
          label="Варианты"
          value={
            variantsTotalQuery.isLoading
              ? null
              : (variantsTotalQuery.data?.total ?? 0)
          }
          icon={<IconDna size={20} />}
          to="/variants"
          hint="Все варианты во всей БД"
        />
        <StatCard
          label="Загруженные образцы"
          value={
            samplesTotalQuery.isLoading
              ? null
              : (samplesTotalQuery.data?.total ?? 0)
          }
          icon={<IconFileUpload size={20} />}
          to="/samples"
          hint="Все загрузки и статусы пайплайна"
        />
      </SimpleGrid>

      <Paper p="md" withBorder>
        <Group justify="space-between" mb="sm">
          <Title order={4}>Последние пациенты</Title>
          <Anchor component={Link} to="/patients" size="sm">
            Все пациенты →
          </Anchor>
        </Group>
        {recentQuery.isError ? (
          <Text c="red">Не удалось загрузить список</Text>
        ) : recentQuery.isLoading ? (
          <Stack>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} h={28} />
            ))}
          </Stack>
        ) : recent.length === 0 ? (
          <Text c="dimmed">
            Пока нет пациентов.{" "}
            <Anchor component={Link} to="/patients/new">
              Создать первого
            </Anchor>
          </Text>
        ) : (
          <Table striped highlightOnHover withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>External ID</Table.Th>
                <Table.Th>ФИО</Table.Th>
                <Table.Th>Дата рождения</Table.Th>
                <Table.Th>Пол</Table.Th>
                <Table.Th>Создан</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {recent.map((p) => (
                <Table.Tr key={p.patient_id}>
                  <Table.Td>{p.external_id ?? "—"}</Table.Td>
                  <Table.Td>
                    <Anchor component={Link} to={`/patients/${p.patient_id}`}>
                      {p.last_name} {p.first_name}
                    </Anchor>
                  </Table.Td>
                  <Table.Td>{formatDate(p.date_of_birth)}</Table.Td>
                  <Table.Td>{p.sex ?? "—"}</Table.Td>
                  <Table.Td>{formatDate(p.created_at)}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Paper>

      <Paper p="md" withBorder>
        <Title order={4} mb="sm">
          Возможности платформы
        </Title>
        <Group gap="xs">
          <Badge variant="light">Аутентификация JWT</Badge>
          <Badge variant="light">CRUD пациентов</Badge>
          <Badge variant="light">Загрузка FASTQ</Badge>
          <Badge variant="light">Пайплайн bwa+samtools+bcftools</Badge>
          <Badge variant="light">Таблица вариантов</Badge>
          <Badge variant="light">Когорта по варианту</Badge>
          <Badge variant="light">Экспорт CSV/JSON</Badge>
        </Group>
      </Paper>
    </Stack>
  );
}

function StatCard({
  label,
  value,
  icon,
  to,
  hint,
}: {
  label: string;
  value: number | string | null;
  icon: React.ReactNode;
  to: string;
  hint?: string;
}) {
  return (
    <Card
      withBorder
      shadow="xs"
      component={Link}
      to={to}
      style={{ textDecoration: "none", color: "inherit" }}
    >
      <Group justify="space-between" mb="xs">
        <Text c="dimmed" size="sm">
          {label}
        </Text>
        {icon}
      </Group>
      {value === null ? (
        <Skeleton h={28} w={80} />
      ) : (
        <Text fw={700} size="xl">
          {value}
        </Text>
      )}
      {hint && (
        <Text size="xs" c="dimmed" mt={4}>
          {hint}
        </Text>
      )}
    </Card>
  );
}
