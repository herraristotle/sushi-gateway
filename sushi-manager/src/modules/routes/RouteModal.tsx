import { useState } from "react";
import Modal from "../../components/layout/Modal";
import { IoMdInformationCircle, IoMdAdd, IoMdClose } from "react-icons/io";
import { useRecoilState } from "recoil";
import { gatewayState } from "../../states/GatewayState";
import AdminApiService from "../../api/services/admin/AdminApiService";
import HttpMethodTag from "./HttpMethodTag";
import Tag from "../../components/typography/Tag";
import JsonView from "react18-json-view";

interface RouteModalProps {
  showModal: boolean;
  onClose: () => void;
  route: any;
}

function RouteModal({ showModal, onClose, route }: RouteModalProps) {
  const [gatewayInfo, setGatewayInfo] = useRecoilState<any>(gatewayState);
  const [newTag, setNewTag] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const handleAddTag = async () => {
    if (!newTag.trim()) return;
    const tag = newTag.trim().toLowerCase();

    if (route.upstream_tags?.includes(tag)) {
      setNewTag("");
      return;
    }

    const updatedTags = [...(route.upstream_tags || []), tag];
    await updateRouteTags(updatedTags);
    setNewTag("");
  };

  const handleRemoveTag = async (tagToRemove: string) => {
    const updatedTags = (route.upstream_tags || []).filter((t: string) => t !== tagToRemove);
    await updateRouteTags(updatedTags);
  };

  const updateRouteTags = async (updatedTags: string[]) => {
    setIsSaving(true);
    try {
      // Create a copy of the route with updated tags
      const updatedRoute = { ...route, upstream_tags: updatedTags };
      // Remove the 'service' field we added in the module
      const { service, ...apiRoute } = updatedRoute;

      await AdminApiService.upsertRoute(service, apiRoute);

      // Update local state
      const newGatewayInfo = JSON.parse(JSON.stringify(gatewayInfo));
      const serviceIdx = newGatewayInfo.gateway.services.findIndex((s: any) => s.name === service);
      if (serviceIdx !== -1) {
        const routeIdx = newGatewayInfo.gateway.services[serviceIdx].routes.findIndex((r: any) => r.name === route.name);
        if (routeIdx !== -1) {
          newGatewayInfo.gateway.services[serviceIdx].routes[routeIdx] = updatedRoute;
          setGatewayInfo(newGatewayInfo);
        }
      }
    } catch (error) {
      console.error("Failed to update route tags:", error);
    } finally {
      setIsSaving(false);
    }
  };
  return (
    <Modal isOpen={showModal} onClose={onClose} title="Route">
      <section className="flex flex-col gap-4 font-lora tracking-wider font-light text-sm">
        <div className="flex gap-2 items-center">
          {route &&
            route.methods.map((method: any, i: number) => {
              return <HttpMethodTag method={method} key={i} />;
            })}
          <span className="font-extralight font-sans tracking-widest text-md">
            {route && route.path}
          </span>
        </div>

        {/* Route Name */}
        <div className="flex gap-2">
          <div className="w-[110px] flex items-center gap-2">
            <span>name</span>
            <IoMdInformationCircle className="text-lg" />
          </div>
          <span>{route && route.name}</span>
        </div>

        {/* Route Service */}
        <div className="flex gap-2">
          <div className="w-[110px] flex items-center gap-2">
            <span>service</span>
            <IoMdInformationCircle className="text-lg" />
          </div>
          <span>{route && route.service}</span>
        </div>

        {/* Route Plugins */}
        <div className="flex gap-2">
          <div className="w-[105px] flex items-center gap-2">
            <span>plugins</span>
            <IoMdInformationCircle className="text-lg" />
          </div>
          <ul className="flex gap-3">
            {route && route.plugins.length > 0 ? (
              route.plugins.map((plugin: any, i: number) => {
                return (
                  <li key={i}>
                    <Tag value={plugin.name} />
                  </li>
                );
              })
            ) : (
              <li>
                <Tag value="None" />
              </li>
            )}
          </ul>
        </div>

        {/* Upstream Tags Section */}
        <div className="mt-2 flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <span className="font-semibold uppercase text-[10px] tracking-widest text-gray-500">Upstream Tags (Subset LB)</span>
            <IoMdInformationCircle className="text-lg text-gray-400" />
          </div>

          <div className="flex flex-wrap gap-2 mt-1 mb-2">
            {route.upstream_tags && route.upstream_tags.length > 0 ? (
              route.upstream_tags.map((tag: string, i: number) => (
                <span key={i} className="bg-purple-50 text-purple-700 text-xs px-2.5 py-1 rounded-lg border border-purple-100 uppercase tracking-wide font-sans font-medium flex items-center gap-1 group">
                  {tag}
                  <button
                    onClick={() => handleRemoveTag(tag)}
                    className="hover:text-red-500 transition-colors"
                  >
                    <IoMdClose size={14} />
                  </button>
                </span>
              ))
            ) : (
              <p className="text-gray-400 italic text-sm">No routing constraints (targets any upstream)</p>
            )}
          </div>

          <div className="flex gap-2">
            <input
              type="text"
              placeholder="Require tag..."
              className="flex-1 px-3 py-1.5 border rounded text-sm font-lora focus:outline-none focus:ring-1 focus:ring-purple-500"
              value={newTag}
              onChange={(e) => setNewTag(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddTag()}
              disabled={isSaving}
            />
            <button
              onClick={handleAddTag}
              disabled={isSaving || !newTag.trim()}
              className="p-2 bg-purple-600 text-white rounded hover:bg-purple-700 disabled:opacity-50 transition-colors"
            >
              <IoMdAdd size={20} />
            </button>
          </div>
          {isSaving && <p className="text-[10px] text-purple-500 mt-1 animate-pulse">Updating route constraints...</p>}
        </div>

        {/* Route Configuration */}
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <span>configuration json</span>
          </div>
          <JsonView style={{ fontSize: "11px" }} src={route} />
        </div>
      </section>
    </Modal>
  );
}

export default RouteModal;
