import { useState, useEffect } from "react";
import { IoMdInformationCircle } from "react-icons/io";
import ServiceModal from "./ServiceModal";
import AdminApiService from "../../api/services/admin/AdminApiService";

interface ServiceTableProps {
  services: any;
}

interface UpstreamStats {
  service_name: string;
  upstreams_count: number;
  healthy_count: number;
  unhealthy_count: number;
}

function ServiceTable({ services }: ServiceTableProps) {
  const [upstreamStats, setUpstreamStats] = useState<Map<string, UpstreamStats>>(new Map());

  useEffect(() => {
    fetchUpstreamStats();
  }, []);

  async function fetchUpstreamStats() {
    try {
      const response = await AdminApiService.getStats();
      const data = response.data;

      // Aggregate stats per service
      const stats = new Map<string, UpstreamStats>();
      data.upstreams?.forEach((u: any) => {
        if (!stats.has(u.service_name)) {
          stats.set(u.service_name, {
            service_name: u.service_name,
            upstreams_count: 0,
            healthy_count: 0,
            unhealthy_count: 0,
          });
        }
        const stat = stats.get(u.service_name)!;
        stat.upstreams_count++;
        if (u.health_status === "healthy") {
          stat.healthy_count++;
        } else if (u.health_status === "unhealthy") {
          stat.unhealthy_count++;
        }
      });
      setUpstreamStats(stats);
    } catch (error) {
      console.error("Failed to fetch upstream stats:", error);
    }
  }

  return (
    <table className="w-full text-sm text-left rtl:text-right">
      <thead className="text-xs uppercase">
        <tr className="font-lora font-light tracking-widest">
          <th className="pl-0 px-6 py-3">
            <div className="flex flex-row items-center gap-2">
              <span>name</span>
              <IoMdInformationCircle className="text-lg mb-0.5" />
            </div>
          </th>
          <th className="px-6 py-3">
            <div className="flex flex-row items-center gap-2">
              <span>base path</span>
              <IoMdInformationCircle className="text-lg mb-0.5" />
            </div>
          </th>
          <th className="px-6 py-3">
            <div className="flex flex-row items-center gap-2">
              <span>protocol</span>
              <IoMdInformationCircle className="text-lg mb-0.5" />
            </div>
          </th>
          <th className="px-6 py-3">
            <div className="flex flex-row items-center gap-2">
              <span>load balancing</span>
              <IoMdInformationCircle className="text-lg mb-0.5" />
            </div>
          </th>
          <th className="px-6 py-3">
            <div className="flex flex-row items-center gap-2">
              <span>upstreams</span>
              <IoMdInformationCircle className="text-lg mb-0.5" />
            </div>
          </th>
          <th className="px-6 py-3">
            <div className="flex flex-row items-center gap-2">
              <span>health</span>
              <IoMdInformationCircle className="text-lg mb-0.5" />
            </div>
          </th>
        </tr>
      </thead>
      <tbody className="font-lora tracking-wider">
        {services?.map((service: any, i: number) => {
          return (
            <ServiceTableRow
              key={i}
              service={service}
              stats={upstreamStats.get(service.name)}
            />
          );
        })}
      </tbody>
    </table>
  );
}

interface ServiceTableRowProps {
  service: any;
  stats?: UpstreamStats;
}

function ServiceTableRow({ service, stats }: ServiceTableRowProps) {
  const [showModal, setShowModal] = useState<boolean>(false);

  const getHealthBadge = () => {
    if (!stats || stats.upstreams_count === 0) {
      return <span className="text-gray-400 text-xs">N/A</span>;
    }

    const healthyPercent = (stats.healthy_count / stats.upstreams_count) * 100;

    if (healthyPercent === 100) {
      return (
        <span className="inline-flex items-center px-2 py-1 rounded-full text-xs bg-green-100 text-green-800 border border-green-300">
          <span className="w-1.5 h-1.5 rounded-full bg-green-500 mr-1.5 animate-pulse"></span>
          All Healthy
        </span>
      );
    } else if (healthyPercent >= 50) {
      return (
        <span className="inline-flex items-center px-2 py-1 rounded-full text-xs bg-yellow-100 text-yellow-800 border border-yellow-300">
          <span className="w-1.5 h-1.5 rounded-full bg-yellow-500 mr-1.5"></span>
          Degraded
        </span>
      );
    } else {
      return (
        <span className="inline-flex items-center px-2 py-1 rounded-full text-xs bg-red-100 text-red-800 border border-red-300">
          <span className="w-1.5 h-1.5 rounded-full bg-red-500 mr-1.5"></span>
          Unhealthy
        </span>
      );
    }
  };

  return (
    <>
      {showModal && (
        <ServiceModal
          showModal={showModal}
          onClose={() => setShowModal(false)}
          service={service}
        />
      )}
      <tr
        className="border-b cursor-pointer transition-all duration-75 hover:bg-gray-100"
        onClick={() => setShowModal(true)}
      >
        <td className="pl-0 px-6 py-4 font-medium whitespace-nowrap">
          {!!service.name && service.name}
        </td>

        <td scope="row" className="px-6 py-4 font-medium whitespace-nowrap">
          {!!service.base_path && service.base_path}
        </td>

        <td scope="row" className="px-6 py-4 font-medium whitespace-nowrap">
          {!!service.protocol && service.protocol}
        </td>

        <td scope="row" className="px-6 py-4 font-medium whitespace-nowrap">
          {!!service.load_balancing_strategy && service.load_balancing_strategy}
        </td>

        <td scope="row" className="px-6 py-4 font-medium whitespace-nowrap">
          {stats?.upstreams_count || 0}
        </td>

        <td scope="row" className="px-6 py-4">
          {getHealthBadge()}
        </td>
      </tr>
    </>
  );
}

export default ServiceTable;
