import {
  AppShell as MantineAppShell,
  Burger,
  Group,
  NavLink,
  Text,
  Button,
  Avatar,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconUsers,
  IconLogout,
  IconDna,
  IconLayoutDashboard,
  IconFileUpload,
} from "@tabler/icons-react";
import { Outlet, useNavigate, useMatch } from "react-router-dom";
import { useAuth } from "../auth/useAuth";

export function AppShell() {
  const [opened, { toggle }] = useDisclosure();
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  // Определяем активность пунктов меню через useMatch
  const dashboardMatch = useMatch("/dashboard");
  const patientsMatch = useMatch("/patients/*");
  const variantsMatch = useMatch("/variants");
  const samplesMatch = useMatch("/samples");

  function handleLogout() {
    logout();
    navigate("/login", { replace: true });
  }

  return (
    <MantineAppShell
      header={{ height: 56 }}
      navbar={{ width: 240, breakpoint: "sm", collapsed: { mobile: !opened } }}
      padding="md"
    >
      <MantineAppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Group>
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" />
            <IconDna size={22} />
            <Text fw={600}>Геномная клиническая платформа</Text>
          </Group>
          <Group gap="sm">
            {user && (
              <Group gap="xs">
                <Avatar radius="xl" size="sm" color="blue">
                  {user.username.slice(0, 2).toUpperCase()}
                </Avatar>
                <Text size="sm">{user.username}</Text>
              </Group>
            )}
            <Button
              variant="subtle"
              size="xs"
              leftSection={<IconLogout size={16} />}
              onClick={handleLogout}
            >
              Выйти
            </Button>
          </Group>
        </Group>
      </MantineAppShell.Header>

      <MantineAppShell.Navbar p="sm">
        <NavLink
          label="Дашборд"
          leftSection={<IconLayoutDashboard size={18} />}
          active={!!dashboardMatch}
          onClick={() => navigate("/dashboard")}
          style={{ borderRadius: 6 }}
        />
        <NavLink
          label="Пациенты"
          leftSection={<IconUsers size={18} />}
          active={!!patientsMatch}
          onClick={() => navigate("/patients")}
          style={{ borderRadius: 6 }}
        />
        <NavLink
          label="Варианты"
          leftSection={<IconDna size={18} />}
          active={!!variantsMatch}
          onClick={() => navigate("/variants")}
          style={{ borderRadius: 6 }}
        />
        <NavLink
          label="Загрузки"
          leftSection={<IconFileUpload size={18} />}
          active={!!samplesMatch}
          onClick={() => navigate("/samples")}
          style={{ borderRadius: 6 }}
        />
      </MantineAppShell.Navbar>

      <MantineAppShell.Main>
        <Outlet />
      </MantineAppShell.Main>
    </MantineAppShell>
  );
}
