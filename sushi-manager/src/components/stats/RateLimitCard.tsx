import { useState, useEffect } from "react";

interface RateLimitStats {
    scope: string;
    hits: number;
    allowed: number;
    hit_rate: number;
}

function RateLimitCard() {
    const [rateLimitStats, setRateLimitStats] = useState<RateLimitStats[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetchRateLimitStats();
        // Refresh every 10 seconds
        const interval = setInterval(fetchRateLimitStats, 10000);
        return () => clearInterval(interval);
    }, []);

    async function fetchRateLimitStats() {
        try {
            const response = await fetch("http://localhost:8001/api/stats");
            const data = await response.json();
            setRateLimitStats(data.rate_limits || []);
        } catch (error) {
            console.error("Failed to fetch rate limit stats:", error);
        } finally {
            setLoading(false);
        }
    }

    const totalHits = rateLimitStats.reduce((sum, stat) => sum + stat.hits, 0);
    const totalAllowed = rateLimitStats.reduce((sum, stat) => sum + stat.allowed, 0);
    const totalRequests = totalHits + totalAllowed;
    const overallHitRate = totalRequests > 0 ? (totalHits / totalRequests) * 100 : 0;

    if (loading) {
        return (
            <div className="bg-white rounded-lg shadow-md p-6 border">
                <div className="animate-pulse">
                    <div className="h-4 bg-gray-200 rounded w-1/3 mb-4"></div>
                    <div className="h-8 bg-gray-200 rounded w-1/2"></div>
                </div>
            </div>
        );
    }

    if (rateLimitStats.length === 0) {
        return (
            <div className="bg-white rounded-lg shadow-md p-6 border">
                <h3 className="text-sm uppercase tracking-widest text-gray-600 mb-2">Rate Limiting</h3>
                <p className="text-xs text-gray-500">No rate limit data available</p>
            </div>
        );
    }

    return (
        <div className="bg-white rounded-lg shadow-md p-6 border">
            <h3 className="text-sm uppercase tracking-widest text-gray-600 mb-4">Rate Limiting Stats</h3>

            {/* Overall Stats */}
            <div className="grid grid-cols-3 gap-4 mb-4">
                <div className="text-center">
                    <div className="text-2xl font-lora text-red-600">{totalHits}</div>
                    <div className="text-xs text-gray-600 mt-1">Hits</div>
                </div>
                <div className="text-center">
                    <div className="text-2xl font-lora text-green-600">{totalAllowed}</div>
                    <div className="text-xs text-gray-600 mt-1">Allowed</div>
                </div>
                <div className="text-center">
                    <div className="text-2xl font-lora text-blue-600">{overallHitRate.toFixed(1)}%</div>
                    <div className="text-xs text-gray-600 mt-1">Hit Rate</div>
                </div>
            </div>

            {/* Per-Scope Stats */}
            <div className="space-y-2 mt-4 border-t pt-4">
                <h4 className="text-xs uppercase tracking-widest text-gray-500 mb-2">By Scope</h4>
                {rateLimitStats.slice(0, 5).map((stat, i) => (
                    <div key={i} className="flex items-center justify-between text-xs">
                        <span className="font-mono text-gray-700 truncate max-w-[150px]" title={stat.scope}>
                            {stat.scope}
                        </span>
                        <div className="flex gap-3">
                            <span className="text-red-600">✗ {stat.hits}</span>
                            <span className="text-green-600">✓ {stat.allowed}</span>
                            <span className="text-gray-500">{stat.hit_rate.toFixed(1)}%</span>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}

export default RateLimitCard;
