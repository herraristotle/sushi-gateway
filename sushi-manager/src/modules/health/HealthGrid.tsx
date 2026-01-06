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

interface HealthGridProps {
    serviceGroups: Record<string, HealthData[]>;
}

function HealthGrid({ serviceGroups }: HealthGridProps) {
    const getStatusColor = (status: string) => {
        switch (status.toLowerCase()) {
            case "healthy":
                return "bg-green-500";
            case "unhealthy":
                return "bg-red-500";
            default:
                return "bg-gray-400";
        }
    };

    const getCheckTypeBadge = (checkType: string) => {
        const colors = {
            active: "bg-blue-100 text-blue-800 border-blue-300",
            passive: "bg-purple-100 text-purple-800 border-purple-300",
            both: "bg-indigo-100 text-indigo-800 border-indigo-300",
            unknown: "bg-gray-100 text-gray-800 border-gray-300",
        };

        return (
            <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs border ${colors[checkType as keyof typeof colors] || colors.unknown}`}>
                {checkType}
            </span>
        );
    };

    const formatLastChecked = (timestamp: string) => {
        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffSec = Math.floor(diffMs / 1000);

        if (diffSec < 60) return `${diffSec}s ago`;
        if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
        return `${Math.floor(diffSec / 3600)}h ago`;
    };

    return (
        <div className="space-y-6">
            {Object.entries(serviceGroups).map(([serviceName, upstreams]) => (
                <div key={serviceName} className="border rounded-lg overflow-hidden">
                    <div className="bg-gray-50 px-4 py-3 border-b">
                        <h3 className="font-lora text-lg tracking-wide">{serviceName}</h3>
                        <p className="text-xs text-gray-600 mt-1">{upstreams.length} upstream{upstreams.length !== 1 ? 's' : ''}</p>
                    </div>

                    <div className="divide-y">
                        {upstreams.map((health) => (
                            <div
                                key={health.upstream_id}
                                className="p-4 hover:bg-gray-50 transition-colors duration-150"
                            >
                                <div className="flex items-center justify-between">
                                    <div className="flex items-center gap-4 flex-1">
                                        {/* Status Indicator */}
                                        <div className="flex items-center gap-2">
                                            <div className={`w-3 h-3 rounded-full ${getStatusColor(health.status)} ${health.status === 'healthy' ? 'animate-pulse' : ''}`}></div>
                                            <span className="font-medium text-sm capitalize">{health.status.replace(/_/g, ' ')}</span>
                                        </div>

                                        {/* Target */}
                                        <div className="flex-1">
                                            <span className="font-mono text-sm text-gray-700">{health.target}</span>
                                        </div>

                                        {/* Check Type */}
                                        <div>
                                            {getCheckTypeBadge(health.check_type)}
                                        </div>

                                        {/* Success/Failure Stats */}
                                        <div className="flex gap-4 text-xs">
                                            <span className="text-green-600">✓ {health.successes}</span>
                                            <span className="text-red-600">✗ {health.failures}</span>
                                        </div>

                                        {/* Last Checked */}
                                        <div className="text-xs text-gray-500">
                                            {formatLastChecked(health.last_checked)}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            ))}
        </div>
    );
}

export default HealthGrid;
