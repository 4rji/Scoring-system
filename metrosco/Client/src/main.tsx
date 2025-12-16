import ReactDOM from "react-dom/client";
import App from "./Pages/App";
import "./index.css";

import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import axios from "axios";
import { QueryClient, QueryClientProvider } from "react-query";
import TeamScore from "./Pages/TeamScore";
import AdminPage from "./Pages/Admin";
import TeamInject from "./Pages/TeamInject";
import TeamPasswords from "./Pages/TeamPasswords";
import LoginPage from "./Pages/Login";
import { useScoreboardInfo } from "./Hooks/CtrlHooks";

const { protocol, host, hostname } = window.location;
// In production, mirror the current origin (respects HTTPS and any tunnel/port in use).
const prodBase = `${protocol}//${host}/api`;
// In dev, default to the local scoreboard port unless explicitly overridden.
const devBase = import.meta.env.VITE_API_URL ?? `http://${hostname}:8000/api`;

axios.defaults.baseURL = import.meta.env.PROD ? prodBase : devBase;
axios.defaults.headers.post["Content-Type"] = "application/json";
axios.defaults.withCredentials = true

const queryClient = new QueryClient();

const LoginRouter = () => {
  const { info, infoLoading } = useScoreboardInfo()
  return (
    <main>
      <Routes>
        <Route path="/" element={<App />} />
        <Route path="/team/:teamName" element={<TeamScore />} />
        <Route path="/team/:teamName/passwords" element={<TeamPasswords />} />
        <Route path="/team/:teamName/inject/:injectId" element={<TeamInject />} />
        <Route path="/admin" element={<AdminPage />} />
        <Route path="/login" element={<LoginPage />} />
      </Routes>
      {!infoLoading && <div className="fixed text-sm right-2 bottom-1 font-light">Metro CCDC Scoreboard v{info.version}</div>}
    </main>
  )
};

ReactDOM.createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={queryClient}>
    <Router>
      <LoginRouter />
    </Router>
  </QueryClientProvider>
);
