import { useState, useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Separator } from "@/components/ui/separator";
import { ChevronDown, ChevronUp, Copy, Download, QrCode, Smartphone } from "lucide-react";
import { toast } from "sonner";
import QRCode from "react-qr-code";
import api from "@/api/api";
import { RequestsAdvancedOptionsReqEncryptionTypeEnum } from "@/api/client/api";
import { useAppStore } from "@/store/general";
import type { JSX } from "react";

interface FormData {
    device_id: string;
    advanced_options: {
        excluded_domains: string;
        encryption_type: RequestsAdvancedOptionsReqEncryptionTypeEnum;
        excluded_wifi_networks: string;
        payload_removal_disallowed: boolean;
        sign_configuration_profile: boolean;
    };
}

export default function MobileconfigGenerator(): JSX.Element {
    const location = useLocation();
    const navigate = useNavigate();
    // Get platform from router state (default to 'ios')
    const platform = location.state?.platform || 'iOS';

    const handleBack = () => {
        navigate('/setup', { state: { platform, fromMobileconfig: true } });
    };
    const activeProfile = useAppStore((state) => state.activeProfile);
    const [showAdvancedOptions, setShowAdvancedOptions] = useState(false);
    const [isDownloading, setIsDownloading] = useState(false);
    const [isGeneratingQR, setIsGeneratingQR] = useState(false);
    const [qrCodeUrl, setQrCodeUrl] = useState("");
    const [linkCopied, setLinkCopied] = useState(false);

    const [formData, setFormData] = useState<FormData>({
        device_id: "",
        advanced_options: {
            excluded_domains: "",
            encryption_type: RequestsAdvancedOptionsReqEncryptionTypeEnum.Https,
            excluded_wifi_networks: "",
            payload_removal_disallowed: false,
            sign_configuration_profile: true,
        }
    });

    // Reset QR code when form data changes
    useEffect(() => {
        setQrCodeUrl("");
        setLinkCopied(false);
    }, [formData]);

    // Validate device ID format
    const isValidDeviceId = (deviceId: string) => {
        if (!deviceId.trim()) return true; // Empty is valid (optional field)
        return /^[A-Za-z0-9 -]*$/.test(deviceId);
    };

    const deviceIdValid = isValidDeviceId(formData.device_id);

    const getRequestData = () => {
        const requestData: {
            profile_id: string;
            advanced_options: typeof formData.advanced_options;
            device_id?: string;
        } = {
            profile_id: activeProfile?.profile_id || "",
            advanced_options: formData.advanced_options
        };

        // Only include device_id if it's not empty
        if (formData.device_id.trim()) {
            requestData.device_id = formData.device_id.trim();
        }

        return requestData;
    };

    const downloadMobileConfig = async () => {
        try {
            setIsDownloading(true);

            // Get the mobileconfig data as a blob
            const response = await api.Client.appleMobileconfigApi.apiV1MobileconfigPost(getRequestData(), {
                responseType: 'blob'
            });

            // Create a blob with the correct MIME type
            const blob = new Blob([response.data], { type: 'application/x-apple-aspen-config' });
            const url = window.URL.createObjectURL(blob);

            // Generate the filename with profile ID
            const filename = `modDNS-${activeProfile?.profile_id || 'profile'}.mobileconfig`;

            // Create a download link (works for both iOS and desktop)
            const link = document.createElement('a');
            link.href = url;
            link.setAttribute('download', filename);
            document.body.appendChild(link);
            link.click();
            link.remove();            // Clean up the blob URL after a short delay
            setTimeout(() => window.URL.revokeObjectURL(url), 100);

            toast.success("Configuration profile downloaded successfully.");
        } catch (error) {
            console.error('Download failed:', error);
            toast.error("Failed to download the configuration profile. Please try again.");
        } finally {
            setIsDownloading(false);
        }
    };

    const generateQRCode = async () => {
        try {
            setIsGeneratingQR(true);
            setLinkCopied(false);

            const response = await api.Client.appleMobileconfigApi.apiV1MobileconfigShortPost(getRequestData());

            if (!response.data || !response.data.link) {
                throw new Error('Invalid response from server');
            }

            setQrCodeUrl(response.data.link);
            toast.success("QR code generated successfully.");
        } catch (error) {
            console.error('QR generation failed:', error);
            toast.error("Failed to generate the QR code. Please try again.");
        } finally {
            setIsGeneratingQR(false);
        }
    };

    const copyShortLink = async () => {
        if (!qrCodeUrl) return;

        try {
            await navigator.clipboard.writeText(qrCodeUrl);
            setLinkCopied(true);
            toast.success("Link copied to clipboard!");
            setTimeout(() => setLinkCopied(false), 2000);
        } catch (error) {
            console.error('Copy failed:', error);
            toast.error("Failed to copy link to clipboard");
        }
    };

    const updateFormField = (field: string, value: string | boolean) => {
        if (field === 'device_id') {
            setFormData(prev => ({
                ...prev,
                device_id: value as string
            }));
        } else if (field.startsWith('advanced_options.')) {
            const optionField = field.replace('advanced_options.', '');
            setFormData(prev => ({
                ...prev,
                advanced_options: {
                    ...prev.advanced_options,
                    [optionField]: value
                }
            }));
        }
    };

    return (
        <div className="w-full px-4 sm:px-6 py-6 mt-20 overflow-x-hidden break-words min-w-0 [word-break:break-word]">
            <div className="max-w-[840px] xl:max-w-[760px] 2xl:max-w-[700px] mx-auto space-y-6">
                <div className="flex justify-center mb-6">
                    <Button
                        onClick={handleBack}
                        className="bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:text-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-900)] font-medium"
                        size="lg"
                    >
                        ← Back
                    </Button>
                </div>
                <Card className="bg-[var(--variable-collection-surface)] border-[var(--shadcn-ui-app-border)] shadow-sm">
                    <CardHeader className="text-center space-y-4">
                        <div className="flex items-center justify-center">
                            <div className="w-12 h-12 bg-[var(--variable-collection-surface)] rounded-lg flex items-center justify-center">
                                <Smartphone className="w-6 h-6 text-[var(--tailwind-colors-rdns-600)]" />
                            </div>
                        </div>
                        <div>
                            <CardTitle className="text-xl sm:text-2xl font-bold bg-[var(--variable-collection-surface)] text-[var(--tailwind-colors-slate-50)] text-balance text-center px-2">
                                Set up modDNS on Apple devices
                            </CardTitle>
                            <p className="text-[var(--tailwind-colors-slate-400)] mt-2 text-sm sm:text-base text-balance text-center px-4">
                                Generate a configuration profile (.mobileconfig) and install on any Apple device to set up modDNS on all networks.
                            </p>
                        </div>
                    </CardHeader>

                    <CardContent className="space-y-6 overflow-x-hidden">
                        {/* Active Profile Display */}
                        <div className="space-y-2">
                            <Label className="text-[var(--tailwind-colors-slate-50)] font-medium">Profile</Label>
                            <div className="px-3 py-2 bg-[var(--shadcn-ui-app-background)] border border-[var(--tailwind-colors-slate-600)] rounded-md overflow-hidden">
                                <span className="text-[var(--tailwind-colors-slate-50)] break-all">
                                    {activeProfile ? activeProfile.name : "No profile selected"}
                                </span>
                            </div>
                            <p className="text-sm text-[var(--tailwind-colors-slate-400)]">
                                Using the currently active profile for mobile config generation.
                            </p>
                        </div>

                        {/* Device ID Input */}
                        <div className="space-y-2">
                            <Label htmlFor="device_id" className="text-[var(--tailwind-colors-slate-50)] font-medium">
                                Device Identifier (Optional)
                            </Label>
                            <Input
                                id="device_id"
                                value={formData.device_id}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateFormField('device_id', e.target.value)}
                                placeholder="e.g., iPhone, John's iPad, Office Mac"
                                className={`bg-[var(--shadcn-ui-app-background)] text-[var(--tailwind-colors-slate-50)] ${deviceIdValid
                                    ? 'border-[var(--tailwind-colors-slate-600)]'
                                    : 'border-red-500'
                                    }`}
                                maxLength={50}
                            />
                            {!deviceIdValid && (
                                <p className="text-sm text-red-400">
                                    Only letters, numbers, spaces, and hyphens are allowed.
                                </p>
                            )}
                            <div className="flex justify-between items-start gap-3 w-full flex-wrap">
                                <p className="text-sm text-[var(--tailwind-colors-slate-400)] break-words min-w-0">
                                    Optional device identifier for tracking queries. Only letters, numbers, spaces, and hyphens allowed.
                                    Will be normalized and limited to 16 characters.
                                </p>
                                {formData.device_id && (
                                    <p className="text-xs text-[var(--tailwind-colors-slate-500)] ml-2 shrink-0 break-all">
                                        {formData.device_id.length}/50
                                    </p>
                                )}
                            </div>
                        </div>

                        {/* Advanced Options Toggle */}
                        <div>
                            <Button
                                variant="ghost"
                                onClick={() => setShowAdvancedOptions(!showAdvancedOptions)}
                                className="p-0 h-auto text-[var(--tailwind-colors-slate-300)] hover:text-[var(--tailwind-colors-rdns-600)]"
                            >
                                <span className="mr-2">{showAdvancedOptions ? 'Hide' : 'More'} options</span>
                                {showAdvancedOptions ? (
                                    <ChevronUp className="h-4 w-4" />
                                ) : (
                                    <ChevronDown className="h-4 w-4" />
                                )}
                            </Button>
                        </div>

                        {/* Advanced Options */}
                        {showAdvancedOptions && (
                            <div className="pt-6 space-y-6 border-t border-[var(--tailwind-colors-slate-600)]">
                                {/* Encryption Type */}
                                <div className="space-y-2">
                                    <Label className="text-[var(--tailwind-colors-slate-50)] font-medium">Encryption type</Label>
                                    <Select
                                        value={formData.advanced_options.encryption_type}
                                        onValueChange={(value) => updateFormField('advanced_options.encryption_type', value as RequestsAdvancedOptionsReqEncryptionTypeEnum)}
                                    >
                                        <SelectTrigger className="bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)]">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="!bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)]">
                                            <SelectItem
                                                value={RequestsAdvancedOptionsReqEncryptionTypeEnum.Https}
                                                className="text-[var(--tailwind-colors-slate-50)]"
                                            >
                                                DNS over HTTPS (DoH)
                                            </SelectItem>
                                            <SelectItem
                                                value={RequestsAdvancedOptionsReqEncryptionTypeEnum.Tls}
                                                className="text-[var(--tailwind-colors-slate-50)]"
                                            >
                                                DNS over TLS (DoT)
                                            </SelectItem>
                                        </SelectContent>
                                    </Select>
                                    <p className="text-sm text-[var(--tailwind-colors-slate-400)]">
                                        Select the encryption protocol for secure DNS queries
                                    </p>
                                </div>

                                {/* WiFi Networks */}
                                <div className="space-y-4">
                                    <div className="space-y-2">
                                        <Label className="text-[var(--tailwind-colors-slate-50)] font-medium">Exclude WiFi networks</Label>
                                        <Textarea
                                            value={formData.advanced_options.excluded_wifi_networks}
                                            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => updateFormField('advanced_options.excluded_wifi_networks', e.target.value)}
                                            placeholder="Add network names separated by commas..."
                                            className="bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)]"
                                        />
                                        <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                            Specify WiFi networks where modDNS should be disabled. Leave empty to enable on all networks.
                                        </p>
                                    </div>
                                </div>

                                {/* Security Note */}
                                <div className="space-y-2">
                                    <Label className="text-[var(--tailwind-colors-slate-50)] font-medium">Security</Label>
                                    <div className="p-3 bg-[var(--shadcn-ui-app-background)] border border-[var(--tailwind-colors-slate-600)] rounded-md">
                                        <p className="text-sm text-[var(--tailwind-colors-slate-300)]">
                                            Configuration profiles are always signed for security and authenticity.
                                        </p>
                                    </div>
                                </div>
                            </div>
                        )}

                        {/* QR Code Display */}
                        {qrCodeUrl && (
                            <div className="pt-6 space-y-4 border-t border-[var(--tailwind-colors-slate-600)]">
                                <div className="text-center space-y-4">
                                    <h3 className="text-lg font-medium text-[var(--tailwind-colors-slate-50)]">
                                        Scan QR Code to Install
                                    </h3>
                                    <div className="flex justify-center">
                                        <div className="max-w-full p-4 border border-[var(--tailwind-colors-slate-600)] rounded-lg bg-[var(--shadcn-ui-app-background)]">
                                            <QRCode
                                                value={qrCodeUrl}
                                                fgColor="var(--tailwind-colors-rdns-600)"
                                                bgColor="transparent"
                                                size={160}
                                                className="w-full h-auto max-w-[160px] max-h-[160px]"
                                            />
                                        </div>
                                    </div>
                                    <p className="text-sm text-[var(--tailwind-colors-slate-400)]">
                                        Scan this QR code with your iPhone or iPad camera to install the configuration
                                    </p>
                                </div>

                                {/* Short Link */}
                                <div className="space-y-2">
                                    <Label className="text-[var(--tailwind-colors-slate-50)] text-sm">Share Link</Label>
                                    <div className="flex flex-col sm:flex-row gap-2 w-full overflow-x-auto">
                                        <input
                                            type="text"
                                            readOnly
                                            value={qrCodeUrl}
                                            className="flex-1 px-3 py-2 text-sm bg-[var(--shadcn-ui-app-background)] border border-[var(--tailwind-colors-slate-600)] rounded-md text-[var(--tailwind-colors-slate-300)]"
                                        />
                                        <Button
                                            onClick={copyShortLink}
                                            variant="outline"
                                            size="sm"
                                            className="border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-900)]"
                                        >
                                            <Copy className="w-4 h-4 mr-1" />
                                            {linkCopied ? "Copied!" : "Copy"}
                                        </Button>
                                    </div>
                                    <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                        Or share this link directly (link is valid for 5 minutes)
                                    </p>
                                </div>
                            </div>
                        )}

                        {/* Action Buttons */}
                        <div className="space-y-4">
                            <Button
                                onClick={downloadMobileConfig}
                                disabled={!activeProfile?.profile_id || isDownloading || !deviceIdValid}
                                className="w-full bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:text-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-900)] font-medium"
                                size="lg"
                            >
                                <Download className="w-4 h-4 mr-2" />
                                {isDownloading ? "Downloading..." : "Download Configuration Profile"}
                            </Button>

                            <div className="flex items-center gap-4">
                                <Separator className="flex-1 bg-[var(--tailwind-colors-slate-600)]" />
                                <span className="text-sm text-[var(--tailwind-colors-slate-400)] font-medium">OR</span>
                                <Separator className="flex-1 bg-[var(--tailwind-colors-slate-600)]" />
                            </div>

                            <Button
                                onClick={generateQRCode}
                                disabled={!activeProfile?.profile_id || isGeneratingQR || !deviceIdValid}
                                variant="outline"
                                className="w-full border-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-900)]"
                                size="lg"
                            >
                                <QrCode className="w-4 h-4 mr-2" />
                                {isGeneratingQR ? "Generating QR Code..." : "Generate QR Code"}
                            </Button>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}
