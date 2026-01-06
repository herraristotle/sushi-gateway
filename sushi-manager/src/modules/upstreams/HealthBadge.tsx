interface HealthBadgeProps {
    status: string;
}

function HealthBadge({ status }: HealthBadgeProps) {
    // Map health status to colors
    const getStatusColor = () => {
        switch (status.toLowerCase()) {
            case "healthy":
                return "bg-green-100 text-green-800 border-green-300";
            case "unhealthy":
                return "bg-red-100 text-red-800 border-red-300";
            case "unknown":
            case "not_available":
                return "bg-gray-100 text-gray-800 border-gray-300";
            default:
                return "bg-yellow-100 text-yellow-800 border-yellow-300";
        }
    };

    const displayStatus = status.replace(/_/g, " ");

    return (
        <span
            className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-medium border transition-all duration-200 ${getStatusColor()}`}
        >
            <span className={`w-2 h-2 rounded-full mr-2 ${status.toLowerCase() === "healthy" ? "bg-green-500 animate-pulse" : ""
                }`}></span>
            {displayStatus}
        </span>
    );
}

export default HealthBadge;
