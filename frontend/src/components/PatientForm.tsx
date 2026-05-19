import {
  Button,
  Grid,
  Group,
  Select,
  Stack,
  Textarea,
  TextInput,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { zodResolver } from "mantine-form-zod-resolver";
import { z } from "zod";
import type { PatientCreate } from "../types/api";

const patientSchema = z.object({
  first_name: z.string().min(1, "Обязательное поле"),
  last_name: z.string().min(1, "Обязательное поле"),
  external_id: z.string().optional(),
  date_of_birth: z
    .string()
    .optional()
    .refine(
      (v) => !v || /^\d{4}-\d{2}-\d{2}$/.test(v),
      "Формат YYYY-MM-DD",
    ),
  sex: z.union([z.literal(""), z.literal("male"), z.literal("female"), z.literal("other")]).optional(),
  phone_number: z.string().optional(),
  email: z
    .string()
    .optional()
    .refine(
      (v) => !v || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v),
      "Некорректный email",
    ),
  address: z.string().optional(),
  phenotype_description: z.string().optional(),
});

export type PatientFormValues = z.infer<typeof patientSchema>;

export interface PatientFormProps {
  initialValues?: Partial<PatientFormValues>;
  submitLabel?: string;
  onCancel?: () => void;
  onSubmit: (values: PatientCreate) => Promise<void>;
  submitting?: boolean;
}

export function PatientForm({
  initialValues,
  submitLabel = "Сохранить",
  onCancel,
  onSubmit,
  submitting,
}: PatientFormProps) {
  const form = useForm<PatientFormValues>({
    initialValues: {
      first_name: "",
      last_name: "",
      external_id: "",
      date_of_birth: "",
      sex: "",
      phone_number: "",
      email: "",
      address: "",
      phenotype_description: "",
      ...initialValues,
    },
    validate: zodResolver(patientSchema),
  });

  async function handleSubmit(values: PatientFormValues) {
    const payload: PatientCreate = {
      first_name: values.first_name,
      last_name: values.last_name,
      external_id: values.external_id?.trim() || null,
      date_of_birth: values.date_of_birth?.trim() || null,
      sex: values.sex ? values.sex : null,
      phone_number: values.phone_number?.trim() || null,
      email: values.email?.trim() || null,
      address: values.address?.trim() || null,
      phenotype_description: values.phenotype_description?.trim() || null,
    };
    await onSubmit(payload);
  }

  return (
    <form onSubmit={form.onSubmit(handleSubmit)}>
      <Stack>
        <Grid>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <TextInput
              label="Фамилия"
              required
              {...form.getInputProps("last_name")}
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <TextInput
              label="Имя"
              required
              {...form.getInputProps("first_name")}
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <TextInput
              label="External ID"
              {...form.getInputProps("external_id")}
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <TextInput
              label="Дата рождения"
              placeholder="YYYY-MM-DD"
              {...form.getInputProps("date_of_birth")}
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <Select
              label="Пол"
              clearable
              data={[
                { value: "male", label: "Мужской" },
                { value: "female", label: "Женский" },
                { value: "other", label: "Другое" },
              ]}
              value={form.values.sex || null}
              onChange={(v) => form.setFieldValue("sex", (v ?? "") as PatientFormValues["sex"])}
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <TextInput
              label="Телефон"
              {...form.getInputProps("phone_number")}
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, sm: 6 }}>
            <TextInput label="Email" {...form.getInputProps("email")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <Textarea
              label="Адрес"
              autosize
              minRows={2}
              {...form.getInputProps("address")}
            />
          </Grid.Col>
          <Grid.Col span={12}>
            <Textarea
              label="Описание фенотипа"
              autosize
              minRows={2}
              {...form.getInputProps("phenotype_description")}
            />
          </Grid.Col>
        </Grid>
        <Group justify="flex-end">
          {onCancel && (
            <Button variant="default" onClick={onCancel}>
              Отмена
            </Button>
          )}
          <Button type="submit" loading={submitting}>
            {submitLabel}
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
