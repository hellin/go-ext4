package ext4

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"encoding/binary"

	"github.com/dsoprea/go-logging"
)

const (
	Ext4Magic = 0xef53

	SuperblockSize = 1024

	// The first superblock is after the bootloader code.
	Superblock0Offset = int64(1024)
)

var (
	ErrNotExt4 = errors.New("not ext4")
)

const (
	SbStateCleanlyUnmounted      = 0x1
	SbStateErrorsDetected        = 0x2
	SbStateOrphansBeingRecovered = 0x4
)

const (
	SbErrorsContinue        = 0x1
	SbErrorsRemountReadonly = 0x2
	SbErrorsPanic           = 0x3
)

const (
	SbOsLinux   = 0x0
	SbOsHurd    = 0x1
	SbOsMasix   = 0x2
	SbOsFreebsd = 0x3
	SbOsLites   = 0x4
)

const (
	SbRevlevelGoodOldRev = 0x0
	SbRevlevelDynamicRev = 0x1
)

type Superblock struct {
	// See fs/ext4/ext4.h .

	// 0x00
	SInodesCount       uint32
	SBlocksCountLo     uint32
	SRBlocksCountLo    uint32
	SFreeBlocksCountLo uint32

	// 0x10
	SFreeInodesCount uint32
	SFirstDataBlock  uint32
	SLogBlockSize    uint32
	SLogClusterSize  uint32

	// 0x20
	SBlocksPerGroup   uint32
	SClustersPerGroup uint32
	SInodesPerGroup   uint32
	SMtime            uint32

	// 0x30
	SWtime         uint32
	SMntCount      uint16
	SMaxMntCount   uint16
	SMagic         uint16
	SState         uint16
	SErrors        uint16
	SMinorRevLevel uint16

	// 0x40
	SLastcheck     uint32
	SCheckinterval uint32
	SCreatorOs     uint32
	SRevLevel      uint32

	// 0x50
	SDefResuid uint16
	SDefResgid uint16

	// The below is present only if (`HasExtended()` == true).

	/*
	 * These fields are for EXT4_DYNAMIC_REV superblocks only.
	 *
	 * Note: the difference between the compatible feature set and
	 * the incompatible feature set is that if there is a bit set
	 * in the incompatible feature set that the kernel doesn't
	 * know about, it should refuse to mount the filesystem.
	 *
	 * e2fsck's requirements are more strict; if it doesn't know
	 * about a feature in either the compatible or incompatible
	 * feature set, it must abort and not try to meddle with
	 * things it doesn't understand...
	 */
	SFirstIno      uint32 /* First non-reserved inode */
	SInodeSize     uint16 /* size of inode structure */
	SBlockGroupNr  uint16 /* block group # of this superblock */
	SFeatureCompat uint32 /* compatible feature set */

	// 0x60
	SFeatureIncompat uint32 /* incompatible feature set */
	SFeatureRoCompat uint32 /* readonly-compatible feature set */

	// 0x68
	SUuid [16]uint8 /* 128-bit uuid for volume */

	// 0x78
	SVolumeName [16]byte /* volume name */

	// 0x88
	SLastMounted [64]byte /* directory where last mounted */

	// 0xC8
	SAlgorithmUsageBitmap uint32 /* For compression */

	/*
	 * Performance hints.  Directory preallocation should only
	 * happen if the EXT4_FEATURE_COMPAT_DIR_PREALLOC flag is on.
	 */
	SPreallocBlocks    uint8  /* Nr of blocks to try to preallocate*/
	SPreallocDirBlocks uint8  /* Nr to preallocate for dirs */
	SReservedGdtBlocks uint16 /* Per group desc for online growth */

	// 0xD0
	/*
	 * Journaling support valid if EXT4_FEATURE_COMPAT_HAS_JOURNAL set.
	 */
	SJournalUuid [16]uint8 /* uuid of journal superblock */

	// 0xE0
	SJournalInum    uint32    /* inode number of journal file */
	SJournalDev     uint32    /* device number of journal file */
	SLastOrphan     uint32    /* start of list of inodes to delete */
	SHashSeed       [4]uint32 /* HTREE hash seed */
	SDefHashVersion uint8     /* Default hash version to use */
	SJnlBackupType  uint8
	SDescSize       uint16 /* size of group descriptor */

	// 0x100
	SDefaultMountOpts uint32
	SFirstMetaBg      uint32     /* First metablock block group */
	SMkfsTime         uint32     /* When the filesystem was created */
	SJnlBlocks        [17]uint32 /* Backup of the journal inode */

	// TODO(dustin): Only if EXT4_FEATURE_COMPAT_64BIT.

	/* 64bit support valid if EXT4_FEATURE_COMPAT_64BIT */

	// 0x150
	SBlocksCountHi     uint32 /* Blocks count */
	SRBlocksCountHi    uint32 /* Reserved blocks count */
	SFreeBlocksCountHi uint32 /* Free blocks count */
	SMinExtraIsize     uint16 /* All inodes have at least # bytes */
	SWantExtraIsize    uint16 /* New inodes should reserve # bytes */

	SFlags            uint32 /* Miscellaneous flags */
	SRaidStride       uint16 /* RAID stride */
	SMmpInterval      uint16 /* # seconds to wait in MMP checking */
	SMmpBlock         uint64 /* Block for multi-mount protection */
	SRaidStripeWidth  uint32 /* blocks on all data disks (N*stride)*/
	SLogGroupsPerFlex uint8  /* FLEX_BG group size */
	SChecksumType     uint8  /* metadata checksum algorithm used */
	SEncryptionLevel  uint8  /* versioning level for encryption */
	SReservedPad      uint8  /* Padding to next 32bits */
	SKbytesWritten    uint64 /* nr of lifetime kilobytes written */

	SSnapshotInum         uint32 /* Inode number of active snapshot */
	SSnapshotId           uint32 /* sequential ID of active snapshot */
	SSnapshotRBlocksCount uint64 /* reserved blocks for active snapshot's future use */
	SSnapshotList         uint32 /* inode number of the head of the on-disk snapshot list */

	SErrorCount      uint32    /* number of fs errors */
	SFirstErrorTime  uint32    /* first time an error happened */
	SFirstErrorIno   uint32    /* inode involved in first error */
	SFirstErrorBlock uint64    /* block involved of first error */
	SFirstErrorFunc  [32]uint8 /* function where the error happened */
	SFirstErrorLine  uint32    /* line number where error happened */
	SLastErrorTime   uint32    /* most recent time of an error */
	SLastErrorIno    uint32    /* inode involved in last error */
	SLastErrorLine   uint32    /* line number where error happened */
	SLastErrorBlock  uint64    /* block involved of last error */
	SLastErrorFunc   [32]uint8 /* function where the error happened */

	SMountOpts        [64]uint8
	SUsrQuotaInum     uint32    /* inode for tracking user quota */
	SGrpQuotaInum     uint32    /* inode for tracking group quota */
	SOverheadClusters uint32    /* overhead blocks/clusters in fs */
	SBackupBgs        [2]uint32 /* groups with sparse_super2 SBs */
	SEncryptAlgos     [4]uint8  /* Encryption algorithms in use  */
	SEncryptPwSalt    [16]uint8 /* Salt used for string2key algorithm */
	SLpfIno           uint32    /* Location of the lost+found inode */
	SPrjQuotaInum     uint32    /* inode for tracking project quota */
	SChecksumSeed     uint32    /* crc32c(uuid) if csum_seed set */
	SWtimeHi          uint8
	SMtimeHi          uint8
	SMkfsTimeHi       uint8
	SLastcheckHi      uint8
	SFirstErrorTimeHi uint8
	SLastErrorTimeHi  uint8
	SPad              [2]uint8
	SReserved         [96]uint32 /* Padding to the end of the block */
	SChecksum         int32      /* crc32c(superblock) */
}

func (sb *Superblock) HasExtended() bool {
	return sb.SRevLevel >= SbRevlevelDynamicRev
}

func (sb *Superblock) BlockSize() uint32 {
	return uint32(math.Pow(2, (10 + float64(sb.SLogBlockSize))))
}

func (sb *Superblock) MountTime() time.Time {
	return time.Unix(int64(sb.SMtime), 0)
}

func (sb *Superblock) WriteTime() time.Time {
	return time.Unix(int64(sb.SWtime), 0)
}

func (sb *Superblock) LastCheckTime() time.Time {
	return time.Unix(int64(sb.SLastcheck), 0)
}

func (sb *Superblock) HasCompatibleFeature(mask uint32) bool {
	return (sb.SFeatureCompat & mask) > 0
}

func (sb *Superblock) HasReadonlyCompatibleFeature(mask uint32) bool {
	return (sb.SFeatureRoCompat & mask) > 0
}

func (sb *Superblock) HasIncompatibleFeature(mask uint32) bool {
	return (sb.SFeatureIncompat & mask) > 0
}

func (sb *Superblock) Dump() {
	fmt.Printf("Superblock Info\n")
	fmt.Printf("\n")

	fmt.Printf("SInodesCount: (%d)\n", sb.SInodesCount)
	fmt.Printf("SBlocksCountLo: (%d)\n", sb.SBlocksCountLo)
	fmt.Printf("SRBlocksCountLo: (%d)\n", sb.SRBlocksCountLo)
	fmt.Printf("SFreeBlocksCountLo: (%d)\n", sb.SFreeBlocksCountLo)
	fmt.Printf("SFreeInodesCount: (%d)\n", sb.SFreeInodesCount)
	fmt.Printf("SFirstDataBlock: (%d)\n", sb.SFirstDataBlock)
	fmt.Printf("SLogBlockSize: (%d) => (%d)\n", sb.SLogBlockSize, sb.BlockSize())
	fmt.Printf("SLogClusterSize: (%d)\n", sb.SLogClusterSize)
	fmt.Printf("SBlocksPerGroup: (%d)\n", sb.SBlocksPerGroup)
	fmt.Printf("SClustersPerGroup: (%d)\n", sb.SClustersPerGroup)
	fmt.Printf("SInodesPerGroup: (%d)\n", sb.SInodesPerGroup)
	fmt.Printf("SMtime: [%s]\n", sb.MountTime())
	fmt.Printf("SWtime: [%s]\n", sb.WriteTime())
	fmt.Printf("SMntCount: (%d)\n", sb.SMntCount)
	fmt.Printf("SMaxMntCount: (%d)\n", sb.SMaxMntCount)
	fmt.Printf("SMagic: [%04x]\n", sb.SMagic)
	fmt.Printf("SState: (%04x)\n", sb.SState)
	fmt.Printf("SErrors: (%d)\n", sb.SErrors)
	fmt.Printf("SMinorRevLevel: (%d)\n", sb.SMinorRevLevel)
	fmt.Printf("SLastcheck: [%s]\n", sb.LastCheckTime())
	fmt.Printf("SCheckinterval: (%d)\n", sb.SCheckinterval)
	fmt.Printf("SCreatorOs: (%d)\n", sb.SCreatorOs)
	fmt.Printf("SRevLevel: (%d)\n", sb.SRevLevel)
	fmt.Printf("SDefResuid: (%d)\n", sb.SDefResuid)
	fmt.Printf("SDefResgid: (%d)\n", sb.SDefResgid)

	// TODO(dustin): Finish.

	fmt.Printf("\n")

	fmt.Printf("Feature (Compatible)\n")
	fmt.Printf("\n")

	for _, name := range SbFeatureCompatNames {
		bit := SbFeatureCompatLookup[name]
		fmt.Printf("  %15s (0x%02x): %v\n", name, bit, sb.HasCompatibleFeature(bit))
	}

	fmt.Printf("\n")

	fmt.Printf("Feature (Read-Only Compatible)\n")
	fmt.Printf("\n")

	for _, name := range SbFeatureRoCompatNames {
		bit := SbFeatureRoCompatLookup[name]
		fmt.Printf("  %15s (0x%02x): %v\n", name, bit, sb.HasReadonlyCompatibleFeature(bit))
	}

	fmt.Printf("\n")

	fmt.Printf("Feature (Incompatible)\n")
	fmt.Printf("\n")

	for _, name := range SbFeatureIncompatNames {
		bit := SbFeatureIncompatLookup[name]
		fmt.Printf("  %15s (0x%02x): %v\n", name, bit, sb.HasIncompatibleFeature(bit))
	}

	fmt.Printf("\n")
}

const (
	SbFeatureCompatDirPrealloc  = uint32(0x0001)
	SbFeatureCompatImagicInodes = uint32(0x0002)
	SbFeatureCompatHasJournal   = uint32(0x0004)
	SbFeatureCompatExtAttr      = uint32(0x0008)
	SbFeatureCompatResizeInode  = uint32(0x0010)
	SbFeatureCompatDirIndex     = uint32(0x0020)
)

var (
	SbFeatureCompatNames = []string{
		"DirIndex",
		"DirPrealloc",
		"ExtAttr",
		"HasJournal",
		"ImagicInodes",
		"ResizeInode",
	}

	SbFeatureCompatLookup = map[string]uint32{
		"DirPrealloc":  SbFeatureCompatDirPrealloc,
		"ImagicInodes": SbFeatureCompatImagicInodes,
		"HasJournal":   SbFeatureCompatHasJournal,
		"ExtAttr":      SbFeatureCompatExtAttr,
		"ResizeInode":  SbFeatureCompatResizeInode,
		"DirIndex":     SbFeatureCompatDirIndex,
	}
)

const (
	SbFeatureRoCompatSparseSuper = uint32(0x0001)
	SbFeatureRoCompatLargeFile   = uint32(0x0002)
	SbFeatureRoCompatBtreeDir    = uint32(0x0004)
	SbFeatureRoCompatHugeFile    = uint32(0x0008)
	SbFeatureRoCompatGdtCsum     = uint32(0x0010)
	SbFeatureRoCompatDirNlink    = uint32(0x0020)
	SbFeatureRoCompatExtraIsize  = uint32(0x0040)
)

var (
	SbFeatureRoCompatNames = []string{
		"BtreeDir",
		"DirNlink",
		"ExtraIsize",
		"GdtCsum",
		"HugeFile",
		"LargeFile",
		"SparseSuper",
	}

	SbFeatureRoCompatLookup = map[string]uint32{
		"SparseSuper": SbFeatureRoCompatSparseSuper,
		"LargeFile":   SbFeatureRoCompatLargeFile,
		"BtreeDir":    SbFeatureRoCompatBtreeDir,
		"HugeFile":    SbFeatureRoCompatHugeFile,
		"GdtCsum":     SbFeatureRoCompatGdtCsum,
		"DirNlink":    SbFeatureRoCompatDirNlink,
		"ExtraIsize":  SbFeatureRoCompatExtraIsize,
	}
)

const (
	SbFeatureIncompatCompression = uint32(0x0001)
	SbFeatureIncompatFiletype    = uint32(0x0002)
	SbFeatureIncompatRecover     = uint32(0x0004) /* Needs recovery */
	SbFeatureIncompatJournalDev  = uint32(0x0008) /* Journal device */
	SbFeatureIncompatMetaBg      = uint32(0x0010)
	SbFeatureIncompatExtents     = uint32(0x0040) /* extents support */
	SbFeatureIncompat64bit       = uint32(0x0080)
	SbFeatureIncompatMmp         = uint32(0x0100)
	SbFeatureIncompatFlexBg      = uint32(0x0200)
)

var (
	SbFeatureIncompatNames = []string{
		"64bit",
		"Compression",
		"Extents",
		"Filetype",
		"FlexBg",
		"JournalDev",
		"MetaBg",
		"Mmp",
		"Recover",
	}

	SbFeatureIncompatLookup = map[string]uint32{
		"Compression": SbFeatureIncompatCompression,
		"Filetype":    SbFeatureIncompatFiletype,
		"Recover":     SbFeatureIncompatRecover,
		"JournalDev":  SbFeatureIncompatJournalDev,
		"MetaBg":      SbFeatureIncompatMetaBg,
		"Extents":     SbFeatureIncompatExtents,
		"64bit":       SbFeatureIncompat64bit,
		"Mmp":         SbFeatureIncompatMmp,
		"FlexBg":      SbFeatureIncompatFlexBg,
	}
)

func ParseSuperblock(r io.Reader) (sb *Superblock, err error) {
	defer func() {
		if state := recover(); state != nil {
			err := log.Wrap(state.(error))
			log.Panic(err)
		}
	}()

	sb = new(Superblock)

	err = binary.Read(r, binary.LittleEndian, sb)
	log.PanicIf(err)

	if sb.SMagic != Ext4Magic {
		log.Panic(ErrNotExt4)
	}

	return sb, nil
}
