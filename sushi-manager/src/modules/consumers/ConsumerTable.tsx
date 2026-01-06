import { IoMdInformationCircle } from "react-icons/io";

interface Consumer {
    id: string;
    username: string;
    custom_id?: string;
    created_at: string;
}

interface ConsumerTableProps {
    consumers: Consumer[];
}

function ConsumerTable({ consumers }: ConsumerTableProps) {
    const formatDate = (dateString: string) => {
        if (!dateString) return "N/A";
        const date = new Date(dateString);
        return date.toLocaleDateString() + " " + date.toLocaleTimeString();
    };

    return (
        <table className="w-full text-sm text-left rtl:text-right">
            <thead className="text-xs uppercase">
                <tr className="font-lora font-light tracking-widest">
                    <th className="pl-0 px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>username</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>custom id</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>id</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                    <th className="px-6 py-3">
                        <div className="flex flex-row items-center gap-2">
                            <span>created</span>
                            <IoMdInformationCircle className="text-lg mb-0.5" />
                        </div>
                    </th>
                </tr>
            </thead>
            <tbody className="font-lora tracking-wider">
                {consumers.map((consumer, i) => (
                    <tr
                        key={i}
                        className="border-b hover:bg-gray-100 transition-all duration-75"
                    >
                        <td className="pl-0 px-6 py-4 font-medium whitespace-nowrap">
                            {consumer.username || "—"}
                        </td>
                        <td className="px-6 py-4 font-medium whitespace-nowrap">
                            {consumer.custom_id || "—"}
                        </td>
                        <td className="px-6 py-4 font-mono text-xs whitespace-nowrap">
                            {consumer.id}
                        </td>
                        <td className="px-6 py-4 text-xs whitespace-nowrap text-gray-600">
                            {formatDate(consumer.created_at)}
                        </td>
                    </tr>
                ))}
            </tbody>
        </table>
    );
}

export default ConsumerTable;
