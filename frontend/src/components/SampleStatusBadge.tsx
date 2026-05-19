import { Badge, Tooltip } from "@mantine/core";
import type { SampleProcessingStatus } from "../types/api";

const LABELS: Record<string, { label: string; color: string }> = {
  processing: { label: "Обработка…", color: "yellow" },
  completed: { label: "Готов", color: "green" },
  failed: { label: "Ошибка", color: "red" },
};

export function SampleStatusBadge({
  status,
  failureReason,
}: {
  status: SampleProcessingStatus;
  failureReason?: string | null;
}) {
  const info = LABELS[status] ?? { label: status, color: "gray" };
  const badge = (
    <Badge color={info.color} variant="light" style={{ cursor: failureReason ? "help" : undefined }}>
      {info.label}
    </Badge>
  );
  if (!failureReason) {
    return badge;
  }
  return (
    <Tooltip label={failureReason} multiline w={360} withArrow position="top">
      {badge}
    </Tooltip>
  );
}
