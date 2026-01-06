import { useState, useEffect } from "react";
import Container from "../../components/layout/Container";
import DashboardCard from "../../components/layout/DashboardCard";
import Header from "../../components/typography/Header";
import Subtitle from "../../components/typography/Subtitle";
import ConsumerTable from "./ConsumerTable";

interface Consumer {
    id: string;
    username: string;
    custom_id?: string;
    created_at: string;
}

function ConsumersModule() {
    const [consumers, setConsumers] = useState<Consumer[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetchConsumers();
    }, []);

    async function fetchConsumers() {
        try {
            // For now, consumers would come from gateway API
            // This is a placeholder - you'd need to add /api/consumers endpoint
            const response = await fetch("http://localhost:8001/api/gateway");
            const data = await response.json();

            // Extract consumers if they exist in the config
            const consumersData = data?.gateway?.consumers || [];
            setConsumers(consumersData);
        } catch (error) {
            console.error("Failed to fetch consumers:", error);
            setConsumers([]);
        } finally {
            setLoading(false);
        }
    }

    return (
        <Container>
            <DashboardCard>
                <div className="p-6">
                    <div className="mb-6">
                        <Header text="consumers" align="left" size="sm" />
                        <Subtitle text="Manage API consumers and their authentication credentials." />
                    </div>

                    {loading ? (
                        <div className="text-center py-8 text-gray-500">Loading consumers...</div>
                    ) : consumers.length === 0 ? (
                        <div className="text-center py-12">
                            <div className="text-gray-400 text-sm">No consumers configured</div>
                            <p className="text-xs text-gray-500 mt-2">
                                Consumers can be configured in the gateway YAML or via API
                            </p>
                        </div>
                    ) : (
                        <ConsumerTable consumers={consumers} />
                    )}
                </div>
            </DashboardCard>
            <div className="mb-24" />
        </Container>
    );
}

export default ConsumersModule;
