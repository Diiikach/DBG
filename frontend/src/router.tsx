import { createBrowserRouter, Navigate } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { ProtectedRoute } from "./auth/ProtectedRoute";
import { AuthPage } from "./pages/AuthPage";
import { DashboardPage } from "./pages/DashboardPage";
import { PatientsListPage } from "./pages/PatientsListPage";
import { PatientNewPage } from "./pages/PatientNewPage";
import { PatientDetailPage } from "./pages/PatientDetailPage";
import { VariantsListPage } from "./pages/VariantsListPage";
import { SamplesListPage } from "./pages/SamplesListPage";

export const router = createBrowserRouter([
  // Объединённая страница с вкладками «Вход» / «Регистрация» (ТЗ 4.1.5 п.1).
  { path: "/login", element: <AuthPage /> },
  { path: "/register", element: <AuthPage /> },
  {
    path: "/",
    element: (
      <ProtectedRoute>
        <AppShell />
      </ProtectedRoute>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: "dashboard", element: <DashboardPage /> },
      { path: "patients", element: <PatientsListPage /> },
      { path: "patients/new", element: <PatientNewPage /> },
      { path: "patients/:id", element: <PatientDetailPage /> },
      { path: "variants", element: <VariantsListPage /> },
      { path: "samples", element: <SamplesListPage /> },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
]);
