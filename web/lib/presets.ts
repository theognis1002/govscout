export interface FilterPreset {
  id: string;
  label: string;
  filters: Record<string, string>;
}

export const PRESETS: FilterPreset[] = [
  {
    id: "software-it",
    label: "Software & IT",
    filters: {
      naics_code: "541511,541512,541513,541519,518210,511210",
      active_only: "true",
    },
  },
  {
    id: "all-active",
    label: "All Active",
    filters: {
      active_only: "true",
    },
  },
  {
    id: "cloud-hosting",
    label: "Cloud & Hosting",
    filters: {
      naics_code: "518210",
      active_only: "true",
    },
  },
  {
    id: "rd-tech",
    label: "R&D / Tech",
    filters: {
      naics_code: "541715,334111",
      active_only: "true",
    },
  },
];

export const DEFAULT_PRESET = PRESETS[0];
