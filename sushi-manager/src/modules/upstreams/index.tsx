import { useState, useEffect } from "react";
import Container from "../../components/layout/Container";
import DashboardCard from "../../components/layout/DashboardCard";
import Header from "../../components/typography/Header";
import Subtitle from "../../components/typography/Subtitle";
import UpstreamTable from "./UpstreamTable";

interface UpstreamData {
    upstream_id: string;
    service_name: string;
    target: string;
    weight: number;
    active_connections: number;
    ewma_latency_ms: number;
    health_status: string;
}

function UpstreamsModule() {
    const [upstreams, setUpstreams] = useState<UpstreamData[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetchUpstreams();
    }, []);

    async function fetchUpstreams() {
        try {
            const response = await fetch("http://localhost:8001/api/stats");
            const data = await response.json();
            setUpstreams(data.upstreams || []);
        } catch (error) {
            console.error("Failed to fetch upstreams:", error);
        } finally {
            setLoading(false);
        }
    }

    return (
        <Container>
            <DashboardCard>
                <div className="p-6">
                    <div className="mb-6">
                        <Header text="upstreams" align="left" size="sm" />
                        <Subtitle text="Upstream targets for all services with health status and load metrics." />
                    </div>
                    {loading ? (
                        <div className="text-center py-8 text-gray-500">Loading upstreams...</div>
                    ) : (
                        <UpstreamTable upstreams={upstreams} />
                    )}
                </div>
            </DashboardCard>
            <div className="mb-24" />
        </Container>
    );
}

export default UpstreamsModule;
