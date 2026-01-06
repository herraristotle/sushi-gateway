import { useState } from "react";
import { IoMdAdd, IoMdClose } from "react-icons/io";
import { useRecoilState } from "recoil";
import { gatewayState } from "../../states/GatewayState";
import AdminApiService from "../../api/services/admin/AdminApiService";

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
        tags?: string[];
    };
}

function UpstreamModal({ showModal, onClose, upstream }: UpstreamModalProps) {
    const [gatewayInfo, setGatewayInfo] = useRecoilState<any>(gatewayState);
    const [newTag, setNewTag] = useState("");
    const [isSaving, setIsSaving] = useState(false);

    const handleAddTag = async () => {
        if (!newTag.trim()) return;
        const tag = newTag.trim().toLowerCase();

        // Check if tag already exists
        if (upstream.tags?.includes(tag)) {
            setNewTag("");
            return;
        }

        const updatedTags = [...(upstream.tags || []), tag];
        await updateUpstreamTags(updatedTags);
        setNewTag("");
    };

    const handleRemoveTag = async (tagToRemove: string) => {
        const updatedTags = (upstream.tags || []).filter(t => t !== tagToRemove);
        await updateUpstreamTags(updatedTags);
    };

    const updateUpstreamTags = async (updatedTags: string[]) => {
        setIsSaving(true);
        try {
            // Find the upstream config that contains this target
            const upstreamConfig = gatewayInfo.gateway.upstreams?.find((u: any) =>
                u.upstreams?.some((target: any) => target.upstream_id === upstream.upstream_id || target.target === upstream.target)
            );

            if (!upstreamConfig) {
                console.error("Could not find upstream config for target", upstream.upstream_id);
                return;
            }

            // Create a deep copy of the config to update
            const newConfig = JSON.parse(JSON.stringify(upstreamConfig));
            const target = newConfig.upstreams.find((t: any) => t.upstream_id === upstream.upstream_id || t.target === upstream.target);
            if (target) {
                target.tags = updatedTags;
            }

            await AdminApiService.upsertUpstream(newConfig);

            // Refresh local state for immediate feedback
            const newGatewayInfo = JSON.parse(JSON.stringify(gatewayInfo));
            const idx = newGatewayInfo.gateway.upstreams.findIndex((u: any) => u.id === upstreamConfig.id);
            if (idx !== -1) {
                newGatewayInfo.gateway.upstreams[idx] = newConfig;
                setGatewayInfo(newGatewayInfo);
            }

        } catch (error) {
            console.error("Failed to update upstream tags:", error);
        } finally {
            setIsSaving(false);
        }
    };
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
                        <label className="text-xs uppercase tracking-widest text-gray-600">Metadata Tags</label>
                        <div className="flex flex-wrap gap-2 mt-1 mb-3">
                            {upstream.tags && upstream.tags.length > 0 ? (
                                upstream.tags.map((tag, i) => (
                                    <span key={i} className="bg-blue-50 text-blue-700 text-xs px-2 py-1 rounded-lg border border-blue-100 uppercase tracking-wide font-sans font-medium flex items-center gap-1 group">
                                        {tag}
                                        <button
                                            onClick={() => handleRemoveTag(tag)}
                                            className="hover:text-red-500 transition-colors"
                                            title="Remove tag"
                                        >
                                            <IoMdClose size={14} />
                                        </button>
                                    </span>
                                ))
                            ) : (
                                <p className="text-gray-400 italic text-sm">No tags assigned</p>
                            )}
                        </div>
                        <div className="flex gap-2">
                            <input
                                type="text"
                                placeholder="Add new tag..."
                                className="flex-1 px-3 py-1.5 border rounded text-sm font-lora focus:outline-none focus:ring-1 focus:ring-blue-500"
                                value={newTag}
                                onChange={(e) => setNewTag(e.target.value)}
                                onKeyDown={(e) => e.key === 'Enter' && handleAddTag()}
                                disabled={isSaving}
                            />
                            <button
                                onClick={handleAddTag}
                                disabled={isSaving || !newTag.trim()}
                                className="p-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
                            >
                                <IoMdAdd size={20} />
                            </button>
                        </div>
                        {isSaving && <p className="text-[10px] text-blue-500 mt-1 animate-pulse">Saving changes...</p>}
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
