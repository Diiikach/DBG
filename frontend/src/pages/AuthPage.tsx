import { useEffect, useState } from "react";
import {
  Button,
  Center,
  Paper,
  PasswordInput,
  Stack,
  Tabs,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { zodResolver } from "mantine-form-zod-resolver";
import { z } from "zod";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import axios from "axios";
import { useAuth } from "../auth/useAuth";
import { extractErrorMessage } from "../api/client";

const loginSchema = z.object({
  username: z.string().min(1, "Введите имя пользователя"),
  password: z.string().min(1, "Введите пароль"),
});

const registerSchema = z.object({
  username: z.string().min(1, "Введите имя пользователя"),
  password: z.string().min(6, "Минимум 6 символов"),
});

type LoginValues = z.infer<typeof loginSchema>;
type RegisterValues = z.infer<typeof registerSchema>;

type Mode = "login" | "register";

/**
 * Единая страница аутентификации с вкладками «Вход» и «Регистрация»
 * (требование 4.1.5 ТЗ). Поддерживает оба URL — /login и /register —
 * чтобы существующие ссылки и редиректы продолжили работать.
 */
export function AuthPage() {
  const params = useParams<{ mode?: string }>();
  const location = useLocation() as {
    state?: { from?: { pathname?: string } };
    pathname: string;
  };
  const navigate = useNavigate();
  const { login, register } = useAuth();

  const initialMode: Mode = location.pathname.startsWith("/register")
    ? "register"
    : params.mode === "register"
      ? "register"
      : "login";

  const [mode, setMode] = useState<Mode>(initialMode);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  // При навигации между /login и /register синхронизируем активную вкладку.
  useEffect(() => {
    const next: Mode = location.pathname.startsWith("/register") ? "register" : "login";
    setMode(next);
    setServerError(null);
  }, [location.pathname]);

  const loginForm = useForm<LoginValues>({
    initialValues: { username: "", password: "" },
    validate: zodResolver(loginSchema),
  });

  const registerForm = useForm<RegisterValues>({
    initialValues: { username: "", password: "" },
    validate: zodResolver(registerSchema),
  });

  async function handleLogin(values: LoginValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      await login(values.username, values.password);
      const redirectTo = location.state?.from?.pathname ?? "/patients";
      navigate(redirectTo, { replace: true });
    } catch (err) {
      const status = axios.isAxiosError(err) ? err.response?.status : undefined;
      if (status === 401) setServerError("Неверный логин или пароль");
      else if (status === 403) setServerError("Учётная запись отключена");
      else setServerError(extractErrorMessage(err, "Не удалось войти"));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRegister(values: RegisterValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      await register(values.username, values.password);
      navigate("/patients", { replace: true });
    } catch (err) {
      const status = axios.isAxiosError(err) ? err.response?.status : undefined;
      if (status === 409) setServerError("Пользователь уже существует");
      else setServerError(extractErrorMessage(err, "Не удалось зарегистрироваться"));
    } finally {
      setSubmitting(false);
    }
  }

  function switchMode(next: Mode) {
    setMode(next);
    setServerError(null);
    // Делаем смену вкладки видимой в URL — это удобно для прямых ссылок.
    navigate(next === "login" ? "/login" : "/register", { replace: true });
  }

  return (
    <Center mih="100vh" p="md">
      <Paper p="xl" radius="md" withBorder w={420}>
        <Stack>
          <Title order={2} ta="center">
            Геномная клиническая платформа
          </Title>

          <Tabs
            value={mode}
            onChange={(v) => v && switchMode(v as Mode)}
            variant="pills"
            radius="md"
          >
            <Tabs.List grow>
              <Tabs.Tab value="login">Вход</Tabs.Tab>
              <Tabs.Tab value="register">Регистрация</Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="login" pt="md">
              <form onSubmit={loginForm.onSubmit(handleLogin)}>
                <Stack>
                  <TextInput
                    label="Имя пользователя"
                    autoComplete="username"
                    {...loginForm.getInputProps("username")}
                  />
                  <PasswordInput
                    label="Пароль"
                    autoComplete="current-password"
                    {...loginForm.getInputProps("password")}
                  />
                  {serverError && mode === "login" && (
                    <Text c="red" size="sm">
                      {serverError}
                    </Text>
                  )}
                  <Button type="submit" loading={submitting} fullWidth>
                    Войти
                  </Button>
                </Stack>
              </form>
            </Tabs.Panel>

            <Tabs.Panel value="register" pt="md">
              <form onSubmit={registerForm.onSubmit(handleRegister)}>
                <Stack>
                  <TextInput
                    label="Имя пользователя"
                    autoComplete="username"
                    {...registerForm.getInputProps("username")}
                  />
                  <PasswordInput
                    label="Пароль"
                    autoComplete="new-password"
                    description="Минимум 6 символов"
                    {...registerForm.getInputProps("password")}
                  />
                  {serverError && mode === "register" && (
                    <Text c="red" size="sm">
                      {serverError}
                    </Text>
                  )}
                  <Button type="submit" loading={submitting} fullWidth>
                    Создать аккаунт
                  </Button>
                </Stack>
              </form>
            </Tabs.Panel>
          </Tabs>
        </Stack>
      </Paper>
    </Center>
  );
}
