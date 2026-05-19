import { useState } from "react";
import { Anchor, Breadcrumbs, Paper, Stack, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { PatientForm } from "../components/PatientForm";
import { createPatient } from "../api/patients";
import { extractErrorMessage } from "../api/client";
import type { PatientCreate } from "../types/api";

export function PatientNewPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [submitting, setSubmitting] = useState(false);

  const mutation = useMutation({
    mutationFn: (payload: PatientCreate) => createPatient(payload),
    onSuccess: (patient) => {
      qc.invalidateQueries({ queryKey: ["patients"] });
      notifications.show({
        color: "green",
        title: "Пациент создан",
        message: `${patient.last_name} ${patient.first_name}`,
      });
      navigate(`/patients/${patient.patient_id}`, { replace: true });
    },
    onError: (err) => {
      notifications.show({
        color: "red",
        title: "Не удалось создать пациента",
        message: extractErrorMessage(err),
      });
    },
  });

  async function handleSubmit(values: PatientCreate) {
    setSubmitting(true);
    try {
      await mutation.mutateAsync(values);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Stack>
      <Breadcrumbs>
        <Anchor component={Link} to="/patients">
          Пациенты
        </Anchor>
        <span>Новый пациент</span>
      </Breadcrumbs>
      <Title order={2}>Новый пациент</Title>
      <Paper p="md" withBorder>
        <PatientForm
          onSubmit={handleSubmit}
          submitting={submitting}
          onCancel={() => navigate("/patients")}
          submitLabel="Создать"
        />
      </Paper>
    </Stack>
  );
}
