import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Button,
  Divider,
  FileInput,
  Group,
  Paper,
  Progress,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconAlertCircle, IconUpload } from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { extractErrorMessage } from "../api/client";
import { getSample, listSamples, uploadSample } from "../api/samples";
import type { Sample, SampleUploadResponse } from "../types/api";
import { SampleStatusBadge } from "./SampleStatusBadge";
import { formatDateTime } from "../utils/format";

const SAMPLE_TYPES = [{ value: "short-reads", label: "short-reads" }];
const SEQ_TYPES = [
  { value: "OTHER", label: "OTHER" },
  { value: "WGS", label: "WGS" },
  { value: "WES", label: "WES" },
  { value: "PANEL", label: "PANEL" },
  { value: "TARGETED", label: "TARGETED" },
  { value: "RNA-SEQ", label: "RNA-SEQ" },
];

interface SamplesTabProps {
  patientId: number;
}

export function SamplesTab({ patientId }: SamplesTabProps) {
  const qc = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [sampleName, setSampleName] = useState("");
  const [sampleType, setSampleType] = useState<string>("short-reads");
  const [seqType, setSeqType] = useState<string>("OTHER");
  const [panelName, setPanelName] = useState("");
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [pollingIds, setPollingIds] = useState<Set<number>>(new Set());

  const samplesQuery = useQuery({
    queryKey: ["samples", patientId],
    queryFn: () => listSamples(patientId),
  });

  // Авто-добавляем в poll все образцы, что в processing.
  useEffect(() => {
    if (!samplesQuery.data) return;
    const processing = samplesQuery.data.items
      .filter((s) => s.processing_status === "processing")
      .map((s) => s.sample_id);
    if (processing.length === 0) return;
    setPollingIds((prev) => {
      const next = new Set(prev);
      for (const id of processing) next.add(id);
      return next;
    });
  }, [samplesQuery.data]);

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error("Файл не выбран");
      return uploadSample(
        patientId,
        {
          file,
          sample_name: sampleName.trim() || undefined,
          sample_type: sampleType,
          sequencing_type: seqType,
          panel_name: panelName.trim() || undefined,
        },
        (p) => setUploadProgress(p),
      );
    },
    onSuccess: (resp: SampleUploadResponse) => {
      notifications.show({
        color: "green",
        title: "Файл принят",
        message: `Sample #${resp.sample_id} поставлен в обработку`,
      });
      setFile(null);
      setSampleName("");
      setPanelName("");
      setUploadProgress(null);
      setPollingIds((prev) => new Set(prev).add(resp.sample_id));
      qc.invalidateQueries({ queryKey: ["samples", patientId] });
    },
    onError: (err) => {
      setUploadProgress(null);
      notifications.show({
        color: "red",
        title: "Ошибка загрузки",
        message: extractErrorMessage(err),
        autoClose: 8000,
      });
    },
  });

  return (
    <Stack>
      <Paper p="md" withBorder>
        <Title order={4} mb="sm">
          Загрузка ридов
        </Title>
        <Stack>
          <Group grow>
            <TextInput
              label="Название образца"
              placeholder="Авто, если оставить пустым"
              value={sampleName}
              onChange={(e) => setSampleName(e.currentTarget.value)}
            />
            <Select
              label="Тип образца"
              data={SAMPLE_TYPES}
              value={sampleType}
              onChange={(v) => setSampleType(v ?? "short-reads")}
              allowDeselect={false}
            />
          </Group>
          <Group grow>
            <Select
              label="Тип секвенирования"
              data={SEQ_TYPES}
              value={seqType}
              onChange={(v) => setSeqType(v ?? "OTHER")}
              allowDeselect={false}
            />
            <TextInput
              label="Panel name (не сохраняется на бэке)"
              value={panelName}
              onChange={(e) => setPanelName(e.currentTarget.value)}
            />
          </Group>
          <FileInput
            label="FASTA / FASTQ файл"
            description="Поддерживаются .fa/.fasta/.fq/.fastq, опц. .gz. Лимит ≤ MAX_UPLOAD_MB (256 МБ по умолчанию). Алфавит ридов: A/C/G/T/N, ≤ 2000 нт, ≤ 10000 ридов."
            placeholder="Выберите файл"
            accept=".fa,.fasta,.fq,.fastq,.gz"
            value={file}
            onChange={setFile}
            clearable
          />
          {uploadProgress !== null && uploadProgress < 100 && (
            <Progress value={uploadProgress} animated />
          )}
          <Group justify="flex-end">
            <Button
              leftSection={<IconUpload size={16} />}
              disabled={!file}
              loading={uploadMutation.isPending}
              onClick={() => uploadMutation.mutate()}
            >
              Загрузить
            </Button>
          </Group>
        </Stack>
      </Paper>

      <Divider />

      <Paper p="md" withBorder>
        <Title order={4} mb="sm">
          Образцы пациента
        </Title>
        {samplesQuery.isLoading ? (
          <Text c="dimmed">Загрузка…</Text>
        ) : samplesQuery.isError ? (
          <Alert color="red" icon={<IconAlertCircle size={16} />}>
            Не удалось загрузить список образцов
          </Alert>
        ) : (samplesQuery.data?.items.length ?? 0) === 0 ? (
          <Text c="dimmed">Образцов пока нет</Text>
        ) : (
          <Table striped highlightOnHover withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>ID</Table.Th>
                <Table.Th>Название</Table.Th>
                <Table.Th>Тип</Table.Th>
                <Table.Th>Секвенирование</Table.Th>
                <Table.Th>Создан</Table.Th>
                <Table.Th>Статус</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {samplesQuery.data!.items.map((s) => (
                <SampleRow
                  key={s.sample_id}
                  sample={s}
                  enablePolling={pollingIds.has(s.sample_id)}
                  onCompleted={() => {
                    qc.invalidateQueries({ queryKey: ["samples", patientId] });
                    qc.invalidateQueries({ queryKey: ["variants", patientId] });
                  }}
                />
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Paper>
    </Stack>
  );
}

function SampleRow({
  sample,
  enablePolling,
  onCompleted,
}: {
  sample: Sample;
  enablePolling: boolean;
  onCompleted: () => void;
}) {
  const shouldPoll = enablePolling && sample.processing_status === "processing";

  const pollQuery = useQuery({
    queryKey: ["sample", sample.sample_id],
    queryFn: () => getSample(sample.sample_id),
    enabled: shouldPoll,
    refetchInterval: shouldPoll ? 3000 : false,
  });

  const live = pollQuery.data ?? sample;

  // Сообщаем родителю, когда статус изменился.
  const status = live.processing_status;
  const prevStatus = useMemo(() => sample.processing_status, [sample]);
  useEffect(() => {
    if (status !== prevStatus && (status === "completed" || status === "failed")) {
      if (status === "completed") {
        notifications.show({
          color: "green",
          title: "Анализ завершён",
          message: `Sample #${live.sample_id}: ${live.sample_name}`,
        });
      } else {
        notifications.show({
          color: "red",
          title: "Анализ не удался",
          message: `Sample #${live.sample_id}: ${live.sample_name}`,
        });
      }
      onCompleted();
    }
  }, [status, prevStatus, live.sample_id, live.sample_name, onCompleted]);

  return (
    <Table.Tr>
      <Table.Td>{live.sample_id}</Table.Td>
      <Table.Td>{live.sample_name}</Table.Td>
      <Table.Td>{live.sample_type}</Table.Td>
      <Table.Td>{live.sequencing_type}</Table.Td>
      <Table.Td>{formatDateTime(live.created_at)}</Table.Td>
      <Table.Td>
        <SampleStatusBadge
          status={live.processing_status}
          failureReason={live.failure_reason}
        />
      </Table.Td>
    </Table.Tr>
  );
}
