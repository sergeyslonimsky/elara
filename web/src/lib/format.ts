import { Format } from "@/gen/elara/config/v1/config_pb";

export function formatLabel(format: Format): string {
	switch (format) {
		case Format.JSON:
			return "JSON";
		case Format.YAML:
			return "YAML";
		case Format.OTHER:
			return "Other";
		default:
			return "";
	}
}

export function formatToLanguage(format: string): string {
	switch (format) {
		case "json":
			return "json";
		case "yaml":
			return "yaml";
		default:
			return "plaintext";
	}
}

export function protoFormatToLanguage(format: Format): string {
	switch (format) {
		case Format.JSON:
			return "json";
		case Format.YAML:
			return "yaml";
		default:
			return "plaintext";
	}
}

export function formatToString(f: Format): string {
	switch (f) {
		case Format.JSON:
			return "json";
		case Format.YAML:
			return "yaml";
		case Format.OTHER:
			return "other";
		default:
			return "auto";
	}
}

export function stringToProtoFormat(s: string): Format {
	switch (s) {
		case "json":
			return Format.JSON;
		case "yaml":
			return Format.YAML;
		case "other":
			return Format.OTHER;
		default:
			return Format.UNSPECIFIED;
	}
}
