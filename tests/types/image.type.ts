export interface ImageSummary {
	id: string;
	repo: string;
	repoTags: string[];
	inUse: boolean;
}

export interface ImageUsageCounts {
	imagesInuse: number;
	imagesUnused: number;
	totalImages: number;
	totalImageSize: number;
}
