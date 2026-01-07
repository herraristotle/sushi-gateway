import { useState, useEffect } from "react";
import Container from "../../components/layout/Container";
import DashboardCard from "../../components/layout/DashboardCard";
import Header from "../../components/typography/Header";
import Subtitle from "../../components/typography/Subtitle";
import UpstreamTable from "./UpstreamTable";
import AdminApiService from "../../api/services/admin/AdminApiService";

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

function UpstreamsModule() {
    const [upstreams, setUpstreams] = useState<UpstreamData[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchTerm, setSearchTerm] = useState("");
    const [activeTag, setActiveTag] = useState<string | null>(null);

    useEffect(() => {
        fetchUpstreams();
    }, []);

    async function fetchUpstreams() {
        try {
            const response = await AdminApiService.getStats();
            const data = response.data;
            setUpstreams(data.upstreams || []);
        } catch (error) {
            console.error("Failed to fetch upstreams:", error);
        } finally {
            setLoading(false);
        }
    }

    const allTags = Array.from(new Set(upstreams.flatMap(u => u.tags || []))).sort();

    const filteredUpstreams = upstreams.filter(u => {
        const matchesSearch = u.target.toLowerCase().includes(searchTerm.toLowerCase()) ||
            u.service_name.toLowerCase().includes(searchTerm.toLowerCase());
        const matchesTag = !activeTag || (u.tags && u.tags.includes(activeTag));
        return matchesSearch && matchesTag;
    });

    return (
        <Container>
            <DashboardCard>
                <div className="p-6">
                    <div className="flex justify-between items-start mb-6">
                        <div>
                            <Header text="upstreams" align="left" size="sm" />
                            <Subtitle text="Upstream targets for all services with health status and load metrics." />
                        </div>
                        <div className="flex flex-col items-end gap-2">
                            <input
                                type="text"
                                placeholder="Search by target or service..."
                                className="px-4 py-2 border rounded-md text-sm font-lora focus:outline-none focus:ring-1 focus:ring-blue-500 w-64"
                                value={searchTerm}
                                onChange={(e) => setSearchTerm(e.target.value)}
                            />
                            {allTags.length > 0 && (
                                <div className="flex flex-wrap gap-1 justify-end max-w-xs">
                                    <button
                                        onClick={() => setActiveTag(null)}
                                        className={`px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-tighter ${!activeTag ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-400 hover:bg-gray-200'}`}
                                    >
                                        All
                                    </button>
                                    {allTags.map(tag => (
                                        <button
                                            key={tag}
                                            onClick={() => setActiveTag(tag === activeTag ? null : tag)}
                                            className={`px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-tighter transition-colors ${tag === activeTag ? 'bg-blue-600 text-white' : 'bg-blue-50 text-blue-600 hover:bg-blue-100'}`}
                                        >
                                            {tag}
                                        </button>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                    {loading ? (
                        <div className="text-center py-8 text-gray-500">Loading upstreams...</div>
                    ) : (
                        <UpstreamTable upstreams={filteredUpstreams} />
                    )}
                </div>
            </DashboardCard>
            <div className="mb-24" />
        </Container>
    );
}

export default UpstreamsModule;
