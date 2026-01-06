import { createBrowserRouter } from "react-router-dom";
import IndexModule from "./modules/index";
import LoginModule from "./modules/login";
import ServicesModule from "./modules/services";
import RoutesModule from "./modules/routes";
import PluginsModule from "./modules/plugins";
import UpstreamsModule from "./modules/upstreams";
import HealthModule from "./modules/health";
import ConsumersModule from "./modules/consumers";

export const router = createBrowserRouter([
  { path: "/login", element: <LoginModule /> },
  { path: "/", element: <IndexModule /> },
  { path: "/services", element: <ServicesModule /> },
  { path: "/routes", element: <RoutesModule /> },
  { path: "/plugins", element: <PluginsModule /> },
  { path: "/upstreams", element: <UpstreamsModule /> },
  { path: "/health", element: <HealthModule /> },
  { path: "/consumers", element: <ConsumersModule /> },
]);
