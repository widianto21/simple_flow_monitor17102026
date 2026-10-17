import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import AppLayout from "./components/AppLayout";
import Dashboard from "./pages/Dashboard";
import Flows from "./pages/Flows";
import FlowMonitor from "./pages/FlowMonitor";
import FlowEditor from "./pages/FlowEditor";
import Runs from "./pages/Runs";
import RunDetail from "./pages/RunDetail";
import Stuck from "./pages/Stuck";
import Alerts from "./pages/Alerts";
import Integrate from "./pages/Integrate";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/flows" element={<Flows />} />
          <Route path="/flows/new/edit" element={<FlowEditor />} />
          <Route path="/flows/:idOrKey" element={<FlowMonitor />} />
          <Route path="/flows/:idOrKey/edit" element={<FlowEditor />} />
          <Route path="/runs" element={<Runs />} />
          <Route path="/runs/:id" element={<RunDetail />} />
          <Route path="/stuck" element={<Stuck />} />
          <Route path="/alerts" element={<Alerts />} />
          <Route path="/integrate" element={<Integrate />} />
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
