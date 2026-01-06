interface UpstreamModalProps {
    showModal: boolean;
    onClose: () => void;
    upstream: {
        upstream_id: string;
        service_name: string;
        target: string;
        weight: number;
        active_connections: number;
        ewma_latency_ms: number;
        health_status: string;
    };
}

function UpstreamModal({ showModal, onClose, upstream }: UpstreamModalProps) {
    if (!showModal) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
            <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4">
                <div className="p-6 border-b">
                    <h2 className="text-2xl font-lora tracking-wide">Upstream Details</h2>
                    <p className="text-sm text-gray-600 mt-1">{upstream.target}</p>
                </div>

                <div className="p-6 space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="text-xs uppercase tracking-widest text-gray-600">Service</label>
                            <p className="font-lora text-lg">{upstream.service_name}</p>
                        </div>
                        <div>
                            <label className="text-xs uppercase tracking-widest text-gray-600">Target</label>
                            <p className="font-lora text-lg">{upstream.target}</p>
                        </div>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="text-xs uppercase tracking-widest text-gray-600">Weight</label>
                            <p className="font-lora text-lg">{upstream.weight}</p>
                        </div>
                        <div>
                            <label className="text-xs uppercase tracking-widest text-gray-600">Health Status</label>
                            <p className="font-lora text-lg capitalize">{upstream.health_status.replace(/_/g, " ")}</p>
                        </div>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="text-xs uppercase tracking-widest text-gray-600">Active Connections</label>
                            <p className="font-lora text-lg">{upstream.active_connections}</p>
                        </div>
                        <div>
                            <label className="text-xs uppercase tracking-widest text-gray-600">EWMA Latency</label>
                            <p className="font-lora text-lg">{upstream.ewma_latency_ms.toFixed(2)} ms</p>
                        </div>
                    </div>

                    <div>
                        <label className="text-xs uppercase tracking-widest text-gray-600">Upstream ID</label>
                        <p className="font-mono text-sm text-gray-700">{upstream.upstream_id}</p>
                    </div>
                </div>

                <div className="p-6 border-t flex justify-end">
                    <button
                        onClick={onClose}
                        className="px-4 py-2 bg-gray-200 hover:bg-gray-300 rounded-md font-lora tracking-wide transition-colors"
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
    );
}

export default UpstreamModal;
