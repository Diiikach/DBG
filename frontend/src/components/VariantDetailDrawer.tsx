import { useState } from "react";
import {
  Badge,
  Button,
  Code,
  Divider,
  Drawer,
  Group,
  Paper,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { IconUsers } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { getVariantDetails } from "../api/variants";
import { formatNumber } from "../utils/format";
import { CohortDrawer } from "./CohortDrawer";

export interface VariantDetailDrawerProps {
  variantId: number | null;
  opened: boolean;
  onClose: () => void;
}

export function VariantDetailDrawer({
  variantId,
  opened,
  onClose,
}: VariantDetailDrawerProps) {
  const [cohortOpen, setCohortOpen] = useState(false);

  const { data, isLoading, isError } = useQuery({
    queryKey: ["variant", variantId],
    queryFn: () => getVariantDetails(variantId!),
    enabled: opened && variantId !== null,
  });

  const title = data ? (
    <Group gap="sm">
      <Title order={4}>
        {data.variant.chromosome}:{data.variant.position}{" "}
        <Code>{data.variant.reference}→{data.variant.alternate}</Code>
      </Title>
      {data.variant.rs_id && <Badge variant="light">{data.variant.rs_id}</Badge>}
      {data.variant.variant_type && (
        <Badge variant="light" color="grape">
          {data.variant.variant_type}
        </Badge>
      )}
    </Group>
  ) : (
    <Title order={4}>Вариант</Title>
  );

  return (
    <>
      <Drawer
        opened={opened}
        onClose={onClose}
        title={title}
        position="right"
        size="xl"
      >
        {isLoading && <DetailSkeleton />}
        {isError && <Text c="red">Не удалось загрузить детали варианта</Text>}
        {data && (
          <Stack>
            <Paper p="sm" withBorder>
              <Group justify="space-between">
                <div>
                  <Text size="sm" c="dimmed">
                    Когорта
                  </Text>
                  <Text fw={600}>
                    {data.patient_count} пациент(а/ов) с этим вариантом
                  </Text>
                </div>
                <Button
                  variant="light"
                  leftSection={<IconUsers size={16} />}
                  onClick={() => setCohortOpen(true)}
                  disabled={data.patient_count === 0}
                >
                  Показать пациентов
                </Button>
              </Group>
            </Paper>

            <SectionAnnotations annotations={data.annotations ?? []} />
            <Divider />
            <SectionGenes genes={data.genes ?? []} />
            <Divider />
            <SectionPhenotypes phenotypes={data.phenotypes ?? []} />
            <Divider />
            <SectionInterpretations interpretations={data.interpretations ?? []} />
          </Stack>
        )}
      </Drawer>

      <CohortDrawer
        variantId={variantId}
        opened={cohortOpen}
        onClose={() => setCohortOpen(false)}
      />
    </>
  );
}

/**
 * Shimmer-скелетон для ленивой загрузки деталей варианта
 * (требование ТЗ 4.1.5: «ленивая загрузка обогащённых данных
 * с shimmer-скелетонами»). Имитирует структуру дровера:
 * карточка-сводка, таблица аннотаций, блок генов.
 */
function DetailSkeleton() {
  return (
    <Stack>
      <Paper p="sm" withBorder>
        <Group justify="space-between">
          <Stack gap={4}>
            <Skeleton h={10} w={80} />
            <Skeleton h={16} w={220} />
          </Stack>
          <Skeleton h={32} w={160} radius="sm" />
        </Group>
      </Paper>

      <div>
        <Skeleton h={18} w={160} mb="xs" />
        <Stack gap={6}>
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} h={22} />
          ))}
        </Stack>
      </div>
      <Divider />

      <div>
        <Skeleton h={18} w={120} mb="xs" />
        <Stack gap="xs">
          {Array.from({ length: 2 }).map((_, i) => (
            <Paper key={i} p="sm" withBorder>
              <Group justify="space-between" mb={6}>
                <Skeleton h={14} w={80} />
                <Skeleton h={12} w={120} />
              </Group>
              <Skeleton h={10} w="80%" mb={4} />
              <Skeleton h={10} w="60%" />
            </Paper>
          ))}
        </Stack>
      </div>
    </Stack>
  );
}

function SectionAnnotations({ annotations }: { annotations: import("../types/api").VariantAnnotation[] }) {
  return (
    <div>
      <Title order={5} mb="xs">
        Аннотации ({annotations.length})
      </Title>
      {annotations.length === 0 ? (
        <Text c="dimmed">Нет аннотаций</Text>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <Table striped withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Consequence</Table.Th>
                <Table.Th>Impact</Table.Th>
                <Table.Th>Transcript</Table.Th>
                <Table.Th>HGVSc</Table.Th>
                <Table.Th>HGVSp</Table.Th>
                <Table.Th>gnomAD</Table.Th>
                <Table.Th>SIFT</Table.Th>
                <Table.Th>PolyPhen</Table.Th>
                <Table.Th>CADD</Table.Th>
                <Table.Th>REVEL</Table.Th>
                <Table.Th>ClinVar</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {annotations.map((a) => (
                <Table.Tr key={a.annotation_id}>
                  <Table.Td>{a.consequence ?? "—"}</Table.Td>
                  <Table.Td>
                    {a.impact ? (
                      <Badge variant="light" color={impactColor(a.impact)}>
                        {a.impact}
                      </Badge>
                    ) : (
                      "—"
                    )}
                  </Table.Td>
                  <Table.Td>{a.transcript_id ?? "—"}</Table.Td>
                  <Table.Td>{a.hgvsc ?? "—"}</Table.Td>
                  <Table.Td>{a.hgvsp ?? "—"}</Table.Td>
                  <Table.Td>{formatNumber(a.gnomad_af, 5)}</Table.Td>
                  <Table.Td>
                    {formatNumber(a.sift_score, 3)}
                    {a.sift_prediction ? ` (${a.sift_prediction})` : ""}
                  </Table.Td>
                  <Table.Td>
                    {formatNumber(a.polyphen_score, 3)}
                    {a.polyphen_prediction ? ` (${a.polyphen_prediction})` : ""}
                  </Table.Td>
                  <Table.Td>{formatNumber(a.cadd_score, 2)}</Table.Td>
                  <Table.Td>{formatNumber(a.revel_score, 3)}</Table.Td>
                  <Table.Td>
                    {a.clinvar_significance ?? "—"}
                    {a.clinvar_id ? ` (${a.clinvar_id})` : ""}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </div>
      )}
    </div>
  );
}

function impactColor(impact: string): string {
  switch (impact.toUpperCase()) {
    case "HIGH":
      return "red";
    case "MODERATE":
      return "orange";
    case "LOW":
      return "yellow";
    case "MODIFIER":
      return "gray";
    default:
      return "gray";
  }
}

function SectionGenes({ genes }: { genes: import("../types/api").Gene[] }) {
  return (
    <div>
      <Title order={5} mb="xs">
        Гены ({genes.length})
      </Title>
      {genes.length === 0 ? (
        <Text c="dimmed">Нет генов</Text>
      ) : (
        <Stack gap="xs">
          {genes.map((g) => (
            <Paper key={g.gene_id} p="sm" withBorder>
              <Group justify="space-between">
                <Text fw={600}>{g.gene_symbol ?? "—"}</Text>
                <Text size="sm" c="dimmed">
                  {g.chromosome ? `${g.chromosome}:` : ""}
                  {g.start_position ?? ""}
                  {g.end_position ? `–${g.end_position}` : ""}
                  {g.strand ? ` (${g.strand})` : ""}
                </Text>
              </Group>
              {g.gene_name && <Text size="sm">{g.gene_name}</Text>}
              {g.gene_description && (
                <Text size="sm" c="dimmed">
                  {g.gene_description}
                </Text>
              )}
              <Group gap="xs" mt={4}>
                {g.ensembl_gene_id && (
                  <Badge variant="outline">Ensembl: {g.ensembl_gene_id}</Badge>
                )}
                {g.ncbi_gene_id && (
                  <Badge variant="outline">NCBI: {g.ncbi_gene_id}</Badge>
                )}
                {g.omim_gene_id && (
                  <Badge variant="outline">OMIM: {g.omim_gene_id}</Badge>
                )}
              </Group>
            </Paper>
          ))}
        </Stack>
      )}
    </div>
  );
}

function SectionPhenotypes({
  phenotypes,
}: {
  phenotypes: import("../types/api").Phenotype[];
}) {
  return (
    <div>
      <Title order={5} mb="xs">
        Фенотипы ({phenotypes.length})
      </Title>
      {phenotypes.length === 0 ? (
        <Text c="dimmed">Нет фенотипов</Text>
      ) : (
        <Stack gap="xs">
          {phenotypes.map((p) => (
            <Paper key={p.phenotype_id} p="sm" withBorder>
              <Text fw={600}>{p.phenotype_name}</Text>
              <Group gap="xs" mt={4}>
                {p.omim_phenotype_id && (
                  <Badge variant="outline">OMIM: {p.omim_phenotype_id}</Badge>
                )}
                {p.orpha_code && (
                  <Badge variant="outline">Orpha: {p.orpha_code}</Badge>
                )}
                {p.inheritance_pattern && (
                  <Badge variant="outline">{p.inheritance_pattern}</Badge>
                )}
              </Group>
              {p.description && (
                <Text size="sm" c="dimmed" mt={4}>
                  {p.description}
                </Text>
              )}
            </Paper>
          ))}
        </Stack>
      )}
    </div>
  );
}

function SectionInterpretations({
  interpretations,
}: {
  interpretations: import("../types/api").VariantInterpretation[];
}) {
  return (
    <div>
      <Title order={5} mb="xs">
        Интерпретации ({interpretations.length})
      </Title>
      {interpretations.length === 0 ? (
        <Text c="dimmed">Нет интерпретаций</Text>
      ) : (
        <Stack gap="xs">
          {interpretations.map((i) => (
            <Paper key={i.interpretation_id} p="sm" withBorder>
              <Group justify="space-between">
                <Badge color="blue">{i.acmg_classification}</Badge>
                <Text size="sm" c="dimmed">
                  user #{i.user_id}
                </Text>
              </Group>
              {i.interpretation_text && (
                <Text size="sm" mt={4}>
                  {i.interpretation_text}
                </Text>
              )}
            </Paper>
          ))}
        </Stack>
      )}
    </div>
  );
}
