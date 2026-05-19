import { useState } from "react";
import {
  ActionIcon,
  Alert,
  Anchor,
  Breadcrumbs,
  Button,
  Group,
  Paper,
  SimpleGrid,
  Skeleton,
  Stack,
  Tabs,
  Text,
  Title,
} from "@mantine/core";
import { modals } from "@mantine/modals";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconEdit,
  IconTrash,
  IconUser,
  IconDna,
  IconUpload,
} from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  deletePatient,
  getPatient,
  updatePatient,
} from "../api/patients";
import { extractErrorMessage } from "../api/client";
import { PatientForm } from "../components/PatientForm";
import { SamplesTab } from "../components/SamplesTab";
import { VariantsTab } from "../components/VariantsTab";
import { formatDate } from "../utils/format";
import type { Patient, PatientCreate } from "../types/api";

export function PatientDetailPage() {
  const { id } = useParams<{ id: string }>();
  const patientId = Number(id);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [isEditing, setIsEditing] = useState(false);

  const patientQuery = useQuery({
    queryKey: ["patient", patientId],
    queryFn: () => getPatient(patientId),
    enabled: Number.isFinite(patientId),
  });

  const updateMutation = useMutation({
    mutationFn: (payload: PatientCreate) => updatePatient(patientId, payload),
    onSuccess: (p) => {
      qc.setQueryData(["patient", patientId], p);
      qc.invalidateQueries({ queryKey: ["patients"] });
      notifications.show({
        color: "green",
        title: "Сохранено",
        message: "Карточка пациента обновлена",
      });
      setIsEditing(false);
    },
    onError: (err) => {
      notifications.show({
        color: "red",
        title: "Не удалось сохранить",
        message: extractErrorMessage(err),
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deletePatient(patientId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["patients"] });
      notifications.show({
        color: "green",
        title: "Пациент удалён",
        message: "Запись успешно удалена",
      });
      navigate("/patients", { replace: true });
    },
    onError: (err) => {
      notifications.show({
        color: "red",
        title: "Не удалось удалить",
        message: extractErrorMessage(err),
      });
    },
  });

  function confirmDelete() {
    modals.openConfirmModal({
      title: "Удалить пациента?",
      children: (
        <Text size="sm">
          Действие необратимо. Все связанные образцы и варианты также станут
          недоступны через карточку пациента.
        </Text>
      ),
      labels: { confirm: "Удалить", cancel: "Отмена" },
      confirmProps: { color: "red" },
      onConfirm: () => deleteMutation.mutate(),
    });
  }

  if (!Number.isFinite(patientId)) {
    return (
      <Alert color="red" icon={<IconAlertCircle size={16} />}>
        Некорректный ID пациента
      </Alert>
    );
  }

  if (patientQuery.isLoading) {
    return (
      <Stack>
        <Skeleton h={40} w={300} />
        <Skeleton h={200} />
      </Stack>
    );
  }

  if (patientQuery.isError || !patientQuery.data) {
    return (
      <Alert color="red" icon={<IconAlertCircle size={16} />}>
        Не удалось загрузить пациента
      </Alert>
    );
  }

  const p = patientQuery.data;
  const fullName = `${p.last_name} ${p.first_name}`;

  return (
    <Stack>
      <Breadcrumbs>
        <Anchor component={Link} to="/patients">
          Пациенты
        </Anchor>
        <span>{fullName}</span>
      </Breadcrumbs>

      <Group justify="space-between">
        <Title order={2}>{fullName}</Title>
        <Group>
          {!isEditing && (
            <Button
              variant="light"
              leftSection={<IconEdit size={16} />}
              onClick={() => setIsEditing(true)}
            >
              Редактировать
            </Button>
          )}
          <ActionIcon
            variant="subtle"
            color="red"
            aria-label="Удалить"
            onClick={confirmDelete}
          >
            <IconTrash size={18} />
          </ActionIcon>
        </Group>
      </Group>

      <Tabs defaultValue="info">
        <Tabs.List>
          <Tabs.Tab value="info" leftSection={<IconUser size={14} />}>
            Информация
          </Tabs.Tab>
          <Tabs.Tab value="samples" leftSection={<IconUpload size={14} />}>
            Образцы
          </Tabs.Tab>
          <Tabs.Tab value="variants" leftSection={<IconDna size={14} />}>
            Варианты
          </Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="info" pt="md">
          {isEditing ? (
            <Paper p="md" withBorder>
              <PatientForm
                initialValues={{
                  first_name: p.first_name,
                  last_name: p.last_name,
                  external_id: p.external_id ?? "",
                  date_of_birth: p.date_of_birth ?? "",
                  sex:
                    p.sex === "male" || p.sex === "female" || p.sex === "other"
                      ? p.sex
                      : "",
                  phone_number: p.phone_number ?? "",
                  email: p.email ?? "",
                  address: p.address ?? "",
                  phenotype_description: p.phenotype_description ?? "",
                }}
                submitting={updateMutation.isPending}
                submitLabel="Сохранить"
                onCancel={() => setIsEditing(false)}
                onSubmit={async (values) => {
                  await updateMutation.mutateAsync(values);
                }}
              />
            </Paper>
          ) : (
            <PatientInfoCard patient={p} />
          )}
        </Tabs.Panel>

        <Tabs.Panel value="samples" pt="md">
          <SamplesTab patientId={patientId} />
        </Tabs.Panel>

        <Tabs.Panel value="variants" pt="md">
          <VariantsTab patientId={patientId} />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}

function PatientInfoCard({ patient }: { patient: Patient }) {
  return (
    <Paper p="md" withBorder>
      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
        <InfoRow label="External ID" value={patient.external_id} />
        <InfoRow label="Дата рождения" value={formatDate(patient.date_of_birth)} />
        <InfoRow label="Пол" value={patient.sex} />
        <InfoRow label="Телефон" value={patient.phone_number} />
        <InfoRow label="Email" value={patient.email} />
        <InfoRow label="Адрес" value={patient.address} />
      </SimpleGrid>
      {patient.phenotype_description && (
        <>
          <Text size="sm" c="dimmed" mt="md">
            Описание фенотипа
          </Text>
          <Text>{patient.phenotype_description}</Text>
        </>
      )}
    </Paper>
  );
}

function InfoRow({ label, value }: { label: string; value?: string | null }) {
  return (
    <div>
      <Text size="sm" c="dimmed">
        {label}
      </Text>
      <Text>{value && value !== "—" ? value : "—"}</Text>
    </div>
  );
}
