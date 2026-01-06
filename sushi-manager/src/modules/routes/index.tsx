import { useEffect, useState } from "react";
import Container from "../../components/layout/Container";
import DashboardCard from "../../components/layout/DashboardCard";
import Header from "../../components/typography/Header";
import Subtitle from "../../components/typography/Subtitle";
import { useGatewayData } from "../../hooks/useGatewayState";
import RouteTable from "./RouteTable";

function RoutesModule() {
  const gatewayInfo = useGatewayData();
  const [routes, setRoutes] = useState<any[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [activeTag, setActiveTag] = useState<string | null>(null);

  useEffect(() => {
    if (gatewayInfo?.gateway) {
      setRoutes(parseRouteData());
    }
  }, [gatewayInfo]);

  function parseRouteData(): any[] {
    const routes: any[] = [];
    if (gatewayInfo?.gateway?.services?.length > 0) {
      gatewayInfo.gateway.services.forEach((service: any) => {
        service?.routes?.forEach((route: any) => {
          routes.push({ ...route, service: service?.name });
        });
      });
    }
    return routes;
  }

  const allUpstreamTags = Array.from(new Set(routes.flatMap(r => r.upstream_tags || []))).sort();

  const filteredRoutes = routes.filter(r => {
    const matchesSearch = r.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.path?.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.service?.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesTag = !activeTag || (r.upstream_tags && r.upstream_tags.includes(activeTag));
    return matchesSearch && matchesTag;
  });

  return (
    <Container>
      <DashboardCard>
        <div className="p-6">
          <div className="flex justify-between items-start mb-6">
            <div>
              <Header text="routes" align="left" size="sm" />
              <Subtitle text="Routes define the different endpoints for each Service." />
            </div>
            <div className="flex flex-col items-end gap-2">
              <input
                type="text"
                placeholder="Search by name, path, or service..."
                className="px-4 py-2 border rounded-md text-sm font-lora focus:outline-none focus:ring-1 focus:ring-purple-500 w-72"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
              {allUpstreamTags.length > 0 && (
                <div className="flex flex-wrap gap-1 justify-end max-w-xs">
                  <button
                    onClick={() => setActiveTag(null)}
                    className={`px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-tighter ${!activeTag ? 'bg-purple-600 text-white' : 'bg-gray-100 text-gray-400 hover:bg-gray-200'}`}
                  >
                    All
                  </button>
                  {allUpstreamTags.map(tag => (
                    <button
                      key={tag}
                      onClick={() => setActiveTag(tag === activeTag ? null : tag)}
                      className={`px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-tighter transition-colors ${tag === activeTag ? 'bg-purple-600 text-white' : 'bg-purple-50 text-purple-600 hover:bg-purple-100'}`}
                    >
                      {tag}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
          <RouteTable routes={filteredRoutes} />
        </div>
      </DashboardCard>
      <div className="mb-24" />
    </Container>
  );
}

export default RoutesModule;
