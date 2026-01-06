import { useState } from "react";
import { IoMdInformationCircle } from "react-icons/io";
import UpstreamModal from "./UpstreamModal";
import HealthBadge from "./HealthBadge";

interface UpstreamData {
    upstream_id: string;
    service_name: string;
    target: string;
    weight: number;
    active_connections: number;
    ewma_latency_ms: number;
    health_status: string;
    tags?: string[];
}

interface UpstreamTableProps {
    upstreams: UpstreamData[];
}

function UpstreamTable({ upstreams }: UpstreamTableProps) {
    return (
        <table className="w-full text-sm text-left rtl:text-right">
            <thead className="text-xs uppercase">
                <tr className="font-lora font-light tracking-widest">
                    <th className="pl-0 px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>service</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>target</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>tags</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>health</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>weight</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>active conns</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>latency (ms)</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                </tr>
            </thead>
            <tbody className="font-lora tracking-wider">
                {upstreams.map((upstream, i) => (
                    <UpstreamTableRow key={i} upstream={upstream} />
                ))}
            </tbody>
        </table>
    );
}

interface UpstreamTableRowProps {
    upstream: UpstreamData;
}

function UpstreamTableRow({ upstream }: UpstreamTableRowProps) {
    const [showModal, setShowModal] = useState<boolean>(false);

    return (
        <>
            {showModal && (
                <UpstreamModal
                    showModal={showModal}
                    onClose={() => setShowModal(false)}
                    upstream={upstream}
                />
            )}
            <tr
                className="border-b cursor-pointer transition-all duration-75 hover:bg-gray-100"
                onClick={() => setShowModal(true)}
            >
                <td className="pl-0 px-6 py-4 font-medium whitespace-nowrap">
                    {upstream.service_name}
                </td>
                <td className="px-6 py-4 font-medium whitespace-nowrap">
                    {upstream.target}
                </td>
                <td className="px-6 py-4 font-medium whitespace-nowrap">
                    <div className="flex gap-1">
                        {upstream.tags && upstream.tags.length > 0 ? (
                            upstream.tags.map((tag, i) => (
                                <span key={i} className="bg-blue-50 text-blue-700 text-[10px] px-1.5 py-0.5 rounded border border-blue-100 uppercase tracking-tighter font-sans">
                                    {tag}
                                </span>
                            ))
                        ) : (
                            <span className="text-gray-400 text-[10px]">-</span>
                        )}
                    </div>
                </td>
                <td className="px-6 py-4">
                    <HealthBadge status={upstream.health_status} />
                </td>
                <td className="px-6 py-4 font-medium whitespace-nowrap">
                    {upstream.weight}
                </td>
                <td className="px-6 py-4 font-medium whitespace-nowrap">
                    {upstream.active_connections}
                </td>
                <td className="px-6 py-4 font-medium whitespace-nowrap">
                    {upstream.ewma_latency_ms.toFixed(2)}
                </td>
            </tr>
        </>
    );
}

export default UpstreamTable;
