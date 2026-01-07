import { useState, useEffect } from "react";
import Container from "../../components/layout/Container";
import DashboardCard from "../../components/layout/DashboardCard";
import Header from "../../components/typography/Header";
import Subtitle from "../../components/typography/Subtitle";
import HealthGrid from "./HealthGrid";
import AdminApiService from "../../api/services/admin/AdminApiService";

interface HealthData {
    upstream_id: string;
    service_name: string;
    target: string;
    status: string;
    check_type: string;
    last_checked: string;
    successes: number;
    failures: number;
}

function HealthModule() {
    const [healthData, setHealthData] = useState<HealthData[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetchHealthData();
        // Refresh every 5 seconds for real-time updates
        const interval = setInterval(fetchHealthData, 5000);
        return () => clearInterval(interval);
    }, []);

    async function fetchHealthData() {
        try {
            const response = await AdminApiService.getHealth();
            const data = response.data;
            setHealthData(data || []);
        } catch (error) {
            console.error("Failed to fetch health data:", error);
        } finally {
            setLoading(false);
        }
    }

    // Group by service
    const serviceGroups = healthData.reduce((acc, health) => {
        if (!acc[health.service_name]) {
            acc[health.service_name] = [];
        }
        acc[health.service_name].push(health);
        return acc;
    }, {} as Record<string, HealthData[]>);

    const totalUpstreams = healthData.length;
    const healthyCount = healthData.filter(h => h.status === "healthy").length;
    const unhealthyCount = healthData.filter(h => h.status === "unhealthy").length;

    return (
        <Container>
            <DashboardCard>
                <div className="p-6">
                    <div className="mb-6">
                        <Header text="health dashboard" align="left" size="sm" />
                        <Subtitle text="Real-time health status monitoring for all upstreams." />
                    </div>

                    {/* Summary Stats */}
                    <div className="grid grid-cols-3 gap-4 mb-6">
                        <div className="bg-gray-50 rounded-lg p-4 border">
                            <div className="text-xs uppercase tracking-widest text-gray-600 mb-1">Total Upstreams</div>
                            <div className="text-2xl font-lora">{totalUpstreams}</div>
                        </div>
                        <div className="bg-green-50 rounded-lg p-4 border border-green-200">
                            <div className="text-xs uppercase tracking-widest text-green-700 mb-1">Healthy</div>
                            <div className="text-2xl font-lora text-green-700">{healthyCount}</div>
                        </div>
                        <div className="bg-red-50 rounded-lg p-4 border border-red-200">
                            <div className="text-xs uppercase tracking-widest text-red-700 mb-1">Unhealthy</div>
                            <div className="text-2xl font-lora text-red-700">{unhealthyCount}</div>
                        </div>
                    </div>

                    {loading ? (
                        <div className="text-center py-8 text-gray-500">Loading health data...</div>
                    ) : (
                        <HealthGrid serviceGroups={serviceGroups} />
                    )}
                </div>
            </DashboardCard>
            <div className="mb-24" />
        </Container>
    );
}

export default HealthModule;
