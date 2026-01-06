import HttpRequest from "../../requests/HttpRequest";

function login(username: string, password: string) {
  return HttpRequest.post(
    "/login",
    {},
    {
      headers: {
        Authorization: `Basic ${btoa(`${username}:${password}`)}`,
      },
      withCredentials: true,
    }
  );
}

function logout() {
  return HttpRequest.delete("/logout", { withCredentials: true });
}

function getGatewayData() {
  return HttpRequest.get("/gateway", {
    withCredentials: true,
  });
}

function getGatewayConfig() {
  return HttpRequest.get("/gateway/config", {
    withCredentials: true,
  });
}

function upsertUpstream(upstream: any) {
  return HttpRequest.post("/upstreams", upstream, {
    withCredentials: true,
  });
}

function upsertRoute(serviceName: string, route: any) {
  return HttpRequest.post(`/services/${serviceName}/routes`, route, {
    withCredentials: true,
  });
}

export default {
  login,
  logout,
  getGatewayData,
  getGatewayConfig,
  upsertUpstream,
  upsertRoute,
};
